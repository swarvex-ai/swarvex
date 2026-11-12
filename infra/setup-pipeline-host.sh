#!/bin/sh
# Prepares a separate server for the test pipeline (run as root on a fresh Debian 13 server).
# Safe to run again.
#
# The pipeline runs other people's code: their tests, merge-train runs and preview builds. On its own server, an
# escape or an abuse report never reaches the platform. This script sets up the base: hardening, Docker, and gVisor
# as a second barrier around every container. The pipeline itself follows in a later step.
#
#   sh infra/setup-pipeline-host.sh
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

echo
echo "== Done. Next: the test pipeline (later step)."
