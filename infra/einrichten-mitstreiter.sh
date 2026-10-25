#!/bin/sh
# Richtet einen Mitstreiter-Rechner (Debian/Ubuntu) für den Runner ein (als root ausführen).
# Legt den unprivilegierten Nutzer `runner` an, baut Runner, Sandbox- und Proxy-Image
# (rootless Podman). Mehrfach ausführbar.
#
#   sh infra/einrichten-mitstreiter.sh /pfad/zum/repo
#
# Danach als runner: swarvex login --plattform https://… [--ca …]   (docs/selbst-hosten.md, Abschnitt 6)
# Später ersetzt der Einzeiler-Installer (P7) dieses Skript für Teilnehmer.
set -eu
REPO=${1:?Pfad zum Repo fehlt}
NUTZER=runner

echo "== 1. Pakete"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq podman uidmap slirp4netns passt git jq rsync curl ca-certificates python3 >/dev/null

echo "== 2. Nutzer $NUTZER (ohne Root-Rechte, eigene subuid-Bereiche, läuft auch ohne Login)"
id "$NUTZER" >/dev/null 2>&1 || useradd -m -s /bin/bash "$NUTZER"
loginctl enable-linger "$NUTZER"
rsync -a --delete --exclude swarvex "$REPO/runner/" "/home/$NUTZER/swarvex/"
chown -R "$NUTZER:$NUTZER" "/home/$NUTZER/swarvex"

echo "== 3. Runner, Proxy- und Sandbox-Image bauen (als $NUTZER)"
su - "$NUTZER" -c '
  set -e
  cd ~/swarvex
  podman run --rm -v "$PWD:/src:Z" -w /src -e CGO_ENABLED=0 -e GOCACHE=/tmp/gc docker.io/library/golang:1 \
    go build -trimpath -ldflags="-s -w" -o swarvex .
  mkdir -p ~/.local/bin && install -m 755 swarvex ~/.local/bin/
  podman build -q -f Containerfile.proxy -t swarvex-proxy . >/dev/null
  cd sandbox && podman build -q -t swarvex-sandbox . >/dev/null
  podman network exists swarvex-intern || podman network create --internal swarvex-intern >/dev/null
  echo "   fertig: $(~/.local/bin/swarvex 2>&1 | head -1)"
'
echo "== Weiter als $NUTZER: swarvex login --plattform https://<plattform> [--ca <ca.crt>]"
