#!/bin/sh
# Prepares a separate server for the test pipeline (run as root on a fresh Debian 13 server).
# Safe to run again.
#
# The pipeline runs other people's code: their tests, merge-train runs and preview builds. On its own server, an
# escape or an abuse report never reaches the platform. This script sets up the base: hardening, Docker, and gVisor
# as a second barrier around every container, then the pipeline jobs check and preview and the tests of merge trains.
# Building and pushing the trains stays on the platform: this server never gets a key that can write to a repository.
#
#   PREVIEW_SYNC_KEY='ssh-ed25519 AAAA… swarvex previews (read-only)' METRICS_KEY='ssh-ed25519 AAAA… swarvex metrics tunnel' \
#     TRAIN_KEY='ssh-ed25519 AAAA… swarvex merge train tests' sh infra/setup-pipeline-host.sh
#
# Before: infra/deploy-pipeline-host.sh <target> copies the code, and /opt/swarvex-data/pipeline.env holds
#   SWARVEX_PLATFORM=https://<platform>   SWARVEX_PIPELINE_SECRET=<PIPELINE_SECRET_CHECK of the platform>
# The platform then gets in its pipeline.env: SWARVEX_JOBS=merge, SWARVEX_PREVIEW_FROM=vorschau@<this server>:
# and SWARVEX_PREVIEW_KEY=<private key matching PREVIEW_SYNC_KEY>; it fetches the built previews read-only. With
# SWARVEX_TRAIN_TESTS_ON=root@<this server> and SWARVEX_TRAIN_TESTS_KEY=<private key matching TRAIN_KEY> it sends its
# merge trains here for testing. infra/connect-pipeline-host.sh does all of this.
set -eu

echo "== 1. Packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get full-upgrade -y -qq >/dev/null
apt-get install -y -qq curl ca-certificates gnupg docker.io git git-lfs jq rsync ufw unattended-upgrades >/dev/null
systemctl enable --now docker >/dev/null
# Bounded container logs
[ -f /etc/docker/daemon.json ] || printf '{ "log-driver": "json-file", "log-opts": { "max-size": "10m", "max-file": "3" } }\n' > /etc/docker/daemon.json

echo "== 2. Hardening"
# Automatic security updates (Debian security archive only)
printf 'APT::Periodic::Update-Package-Lists "1";\nAPT::Periodic::Unattended-Upgrade "1";\n' > /etc/apt/apt.conf.d/20auto-upgrades
systemctl enable --now unattended-upgrades >/dev/null 2>&1 || true
# Firewall: only SSH comes in. The pipeline only connects out (platform API, git hosts, package registries).
ufw default deny incoming >/dev/null
ufw default allow outgoing >/dev/null
ufw allow 22/tcp >/dev/null
ufw --force enable >/dev/null
echo "   firewall: $(ufw status | head -1)"
# SSH: keys only, but only if a key is already installed (otherwise we would lock ourselves out)
if [ -s /root/.ssh/authorized_keys ]; then
  printf 'PasswordAuthentication no\nKbdInteractiveAuthentication no\nPermitRootLogin prohibit-password\n' > /etc/ssh/sshd_config.d/10-swarvex.conf
  sshd -t && systemctl reload ssh
  echo "   ssh: keys only"
else
  echo "   ssh: WARNING no key in /root/.ssh/authorized_keys, password login stays enabled"
fi

echo "== 3. gVisor (runsc) as a Docker runtime"
# gVisor gives every container its own user-space kernel: a kernel exploit in a test lands in gVisor, not in the
# host. Without nested virtualisation (most VPS) it uses the systrap platform, which needs no KVM.
if ! command -v runsc >/dev/null 2>&1; then
  curl -fsSL https://gvisor.dev/archive.key | gpg --dearmor --yes -o /usr/share/keyrings/gvisor-archive-keyring.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/gvisor-archive-keyring.gpg] https://storage.googleapis.com/gvisor/releases release main" > /etc/apt/sources.list.d/gvisor.list
  apt-get update -qq
  apt-get install -y -qq runsc >/dev/null
fi
runsc install >/dev/null 2>&1 || true
systemctl restart docker
docker info --format '{{range $k, $v := .Runtimes}}{{$k}} {{end}}' | grep -qw runsc && echo "   docker runtimes: $(docker info --format '{{range $k, $v := .Runtimes}}{{$k}} {{end}}')"
# Smoke test: the kernel inside a gVisor container is gVisor's, not the host's
docker run --rm --runtime=runsc docker.io/library/alpine:3 dmesg 2>/dev/null | grep -q gVisor && echo "   gvisor: works" || { echo "   gvisor: FAILED"; exit 1; }

echo "== 4. Test pipeline (check and preview; merging stays on the platform)"
ENVF=/opt/swarvex-data/pipeline.env
mkdir -p /opt/swarvex-data /var/lib/swarvex-pipeline/vorschau
chmod 755 /var/lib/swarvex-pipeline /var/lib/swarvex-pipeline/vorschau
if [ -f "$ENVF" ]; then
  # Fixed settings of this server; platform address and key stay as the operator wrote them
  for kv in SWARVEX_JOBS=check,preview SWARVEX_RUNTIME=runsc SWARVEX_WORKDIR=/var/lib/swarvex-pipeline; do
    grep -q "^${kv%%=*}=" "$ENVF" || echo "$kv" >> "$ENVF"
  done
  chmod 600 "$ENVF"
  if [ -d /opt/swarvex/pipeline ]; then
    cd /opt/swarvex
    docker run --rm -v "$PWD:/src" -w /src/pipeline -e CGO_ENABLED=0 -e GOCACHE=/tmp/gc docker.io/library/golang:1 go build -trimpath -ldflags="-s -w" -o swarvex-pipeline .
    install -m 755 pipeline/swarvex-pipeline /usr/local/bin/ && rm -f pipeline/swarvex-pipeline
    install -m 644 infra/systemd/swarvex-check.service infra/systemd/swarvex-check.timer infra/systemd/swarvex-preview.service infra/systemd/swarvex-preview.timer /etc/systemd/system/
    systemctl daemon-reload
    systemctl enable --now swarvex-check.timer swarvex-preview.timer >/dev/null
    echo "   pipeline: check and preview timers on"
  else
    echo "   pipeline: no code in /opt/swarvex yet (infra/deploy-pipeline-host.sh)"
  fi
else
  echo "   pipeline: $ENVF missing, see the header of this script"
fi

echo "== 5. Read-only access for the platform to fetch built previews"
# A user without a shell of its own: its one key may only run rrsync, read-only, in the preview folder.
id vorschau >/dev/null 2>&1 || useradd --system --create-home --home-dir /var/lib/swarvex-vorschau-sync --shell /bin/sh vorschau
if [ -n "${PREVIEW_SYNC_KEY:-}" ]; then
  install -d -m 700 -o vorschau -g vorschau /var/lib/swarvex-vorschau-sync/.ssh
  printf 'restrict,command="/usr/bin/rrsync -ro /var/lib/swarvex-pipeline/vorschau/" %s\n' "$PREVIEW_SYNC_KEY" > /var/lib/swarvex-vorschau-sync/.ssh/authorized_keys
  chown vorschau:vorschau /var/lib/swarvex-vorschau-sync/.ssh/authorized_keys && chmod 600 /var/lib/swarvex-vorschau-sync/.ssh/authorized_keys
  echo "   preview sync: key installed"
else
  echo "   preview sync: PREVIEW_SYNC_KEY not given, unchanged"
fi

echo "== 6. Metrics for the platform's monitoring"
# The node exporter listens on 127.0.0.1 only; the platform reaches it through an SSH tunnel it opens itself
# (infra/monitoring/compose.pipeline-server.yaml). Its one key may forward to that port and nothing else: no shell,
# no other port. This server gets no access to the platform. Pipeline runs add their own metrics as textfiles.
apt-get install -y -qq --no-install-recommends prometheus-node-exporter >/dev/null
mkdir -p /var/lib/swarvex-metrics
chmod 755 /var/lib/swarvex-metrics
printf 'ARGS="--web.listen-address=127.0.0.1:9100 --collector.textfile.directory=/var/lib/swarvex-metrics"\n' > /etc/default/prometheus-node-exporter
systemctl enable prometheus-node-exporter >/dev/null 2>&1
systemctl restart prometheus-node-exporter
id metriken >/dev/null 2>&1 || useradd --system --create-home --home-dir /var/lib/swarvex-metriken --shell /usr/sbin/nologin metriken
if [ -n "${METRICS_KEY:-}" ]; then
  install -d -m 700 -o metriken -g metriken /var/lib/swarvex-metriken/.ssh
  printf 'restrict,port-forwarding,permitopen="127.0.0.1:9100",command="/bin/false" %s\n' "$METRICS_KEY" > /var/lib/swarvex-metriken/.ssh/authorized_keys
  chown metriken:metriken /var/lib/swarvex-metriken/.ssh/authorized_keys && chmod 600 /var/lib/swarvex-metriken/.ssh/authorized_keys
  echo "   metrics: node exporter on 127.0.0.1:9100, tunnel key installed"
else
  echo "   metrics: node exporter on 127.0.0.1:9100, METRICS_KEY not given, tunnel key unchanged"
fi

echo "== 7. Merge train tests for the platform"
# The platform builds and pushes merge trains itself (only it holds keys that write to a repository); their tests run
# here. It sends the train as a git bundle; its key may run exactly one command, which reads the train, fetches main
# with this server's own read access, tests under gVisor and answers with the result. No shell, no forwarding.
# The key lives in its own file (/etc/ssh/authorized_keys.d/root), so this script never touches root's own keys.
cat > /usr/local/bin/swarvex-train-test <<'WRAPPER'
#!/bin/sh
# Forced command of the platform's merge train key (/etc/ssh/authorized_keys.d/root): tests one train, nothing else.
set -a
. /opt/swarvex-data/pipeline.env
set +a
exec /usr/local/bin/swarvex-pipeline train-test
WRAPPER
chmod 755 /usr/local/bin/swarvex-train-test
install -d -m 755 /etc/ssh/authorized_keys.d
printf 'AuthorizedKeysFile .ssh/authorized_keys /etc/ssh/authorized_keys.d/%%u\n' > /etc/ssh/sshd_config.d/20-swarvex-train.conf
if [ -n "${TRAIN_KEY:-}" ]; then
  printf 'restrict,command="/usr/local/bin/swarvex-train-test" %s\n' "$TRAIN_KEY" > /etc/ssh/authorized_keys.d/root
  chmod 644 /etc/ssh/authorized_keys.d/root
  echo "   merge train tests: key installed"
else
  echo "   merge train tests: TRAIN_KEY not given, key unchanged"
fi
sshd -t && systemctl reload ssh

echo
echo "== Done."
