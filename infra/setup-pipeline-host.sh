#!/bin/sh
# Prepares a separate server for the test pipeline (run as root on a fresh Debian 13 server).
# Safe to run again.
#
# The pipeline runs other people's code: their tests, merge-train runs and preview builds. On its own server, an
# escape or an abuse report never reaches the platform. This script sets up the base: hardening, Docker, and gVisor
# as a second barrier around every container, then the pipeline jobs check and preview. Merging stays on the platform:
# this server never gets a key that can write to a repository.
#
#   PREVIEW_SYNC_KEY='ssh-ed25519 AAAA… swarvex previews (read-only)' sh infra/setup-pipeline-host.sh
#
# Before: infra/deploy-pipeline-host.sh <target> copies the code, and /opt/swarvex-data/pipeline.env holds
#   SWARVEX_PLATFORM=https://<platform>   SWARVEX_PIPELINE_SECRET=<PIPELINE_SECRET_CHECK of the platform>
# The platform then gets in its pipeline.env: SWARVEX_JOBS=merge, SWARVEX_PREVIEW_FROM=vorschau@<this server>:
# and SWARVEX_PREVIEW_KEY=<private key matching PREVIEW_SYNC_KEY>; it fetches the built previews read-only.
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

echo
echo "== Done."
