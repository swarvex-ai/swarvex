#!/bin/sh
# Baut und signiert ein Runner-Release und legt es mit dem Einzeiler-Installer in den
# Download-Ordner der Plattform (Caddy liefert /install.sh und /downloads/… aus).
# Als root auf der Plattform ausführen:  sh /opt/swarvex/infra/release.sh
#
# Signatur: ssh-keygen -Y (Namensraum swarvex-release). Der private Schlüssel liegt im
# Labor auf der Plattform; live gehört er auf einen getrennten Build-Rechner bzw. in die CI.
set -eu
CODE=/opt/swarvex
DATEN=/opt/swarvex-daten
DOWNLOADS=$DATEN/downloads
BASIS_HOST=$(grep ^BASIS_HOST= "$CODE/infra/stack/.env" | cut -d= -f2-)
PLATTFORM="https://$BASIS_HOST"
VERSION=${VERSION:-0.1.$(date -u +%Y%m%d%H%M)}

[ -f "$DATEN/release_key" ] || ssh-keygen -q -t ed25519 -N "" -C "swarvex-release" -f "$DATEN/release_key"
chmod 600 "$DATEN/release_key"

ZIEL=$DOWNLOADS/$VERSION
BAU=$(mktemp -d)
trap 'rm -rf "$BAU"' EXIT
mkdir -p "$ZIEL"
for ARCH in amd64 arm64; do
  echo "== Runner $VERSION für linux/$ARCH"
  docker run --rm -v "$CODE/runner:/src:ro" -v "$BAU:/aus" -w /src -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=$ARCH -e GOCACHE=/tmp/gc \
    golang:1 go build -trimpath -ldflags="-s -w" -o /aus/swarvex .
  PAKET=$BAU/paket-$ARCH
  mkdir -p "$PAKET/sandbox"
  cp "$BAU/swarvex" "$CODE/runner/Containerfile.proxy" "$PAKET/"
  cp "$CODE/runner/sandbox/Containerfile" "$PAKET/sandbox/"
  tar -czf "$ZIEL/swarvex-$VERSION-linux-$ARCH.tar.gz" -C "$PAKET" .
done
( cd "$ZIEL" && sha256sum swarvex-*.tar.gz > SHA256SUMS )
ssh-keygen -q -Y sign -f "$DATEN/release_key" -n swarvex-release "$ZIEL/SHA256SUMS"   # erzeugt SHA256SUMS.sig

PUB=$(cut -d' ' -f1-2 "$DATEN/release_key.pub")
sed -e "s#@PLATTFORM@#$PLATTFORM#g" -e "s#@VERSION@#$VERSION#g" -e "s#@PUBKEY@#$PUB#g" \
  "$CODE/runner/install.sh.in" > "$DOWNLOADS/install.sh"
chmod -R a+rX "$DOWNLOADS"
echo "== Release $VERSION bereit: $PLATTFORM/install.sh"
ls -la "$ZIEL"
