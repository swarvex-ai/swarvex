#!/bin/sh
# Sets up a contributor machine (Debian/Ubuntu) for the runner from a source checkout (run as root).
# Creates the unprivileged user `runner` and builds the runner, agent and proxy images (rootless Podman).
# Safe to run again. Normal contributors use the one-line installer instead.
#
#   sh infra/setup-contributor.sh /path/to/repo
#
# Then, as runner: swarvex login --platform https://… [--ca …]   (docs/self-hosting.md, section 6)
set -eu
REPO=${1:?path to the repo is missing}
USER_NAME=runner

echo "== 1. Packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq podman uidmap slirp4netns passt git jq rsync curl ca-certificates python3 >/dev/null

echo "== 2. User $USER_NAME (no root rights, own subuid ranges, runs without login)"
id "$USER_NAME" >/dev/null 2>&1 || useradd -m -s /bin/bash "$USER_NAME"
loginctl enable-linger "$USER_NAME"
rsync -a --delete --exclude swarvex "$REPO/runner/" "/home/$USER_NAME/swarvex/"
chown -R "$USER_NAME:$USER_NAME" "/home/$USER_NAME/swarvex"

echo "== 3. Build runner, proxy and agent image (as $USER_NAME)"
su - "$USER_NAME" -c '
  set -e
  cd ~/swarvex
  podman run --rm -v "$PWD:/src:Z" -w /src -e CGO_ENABLED=0 -e GOCACHE=/tmp/gc docker.io/library/golang:1 \
    go build -trimpath -ldflags="-s -w" -o swarvex .
  mkdir -p ~/.local/bin && install -m 755 swarvex ~/.local/bin/
  podman build -q -f Containerfile.proxy -t swarvex-proxy . >/dev/null
  cd sandbox && podman build -q -t swarvex-agent . >/dev/null
  podman network exists swarvex-intern || podman network create --internal swarvex-intern >/dev/null
  echo "   done: $(~/.local/bin/swarvex 2>&1 | head -1)"
'
echo "== Continue as $USER_NAME: swarvex login --platform https://<platform> [--ca <ca.crt>]"
