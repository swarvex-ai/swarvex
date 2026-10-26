#!/bin/sh
# Builds and signs a runner release and puts it, together with the one-line installer, into the
# platform's download folder (Caddy serves /install.sh and /downloads/…).
# Run as root on the platform:  sh /opt/swarvex/infra/release.sh
#
# Signature: ssh-keygen -Y (namespace swarvex-release). In the lab the private key lives on the
# platform; in production it belongs on a separate build machine or in CI.
set -eu
umask 022   # release files must be readable for everyone (setup-platform.sh calls this with umask 077)
CODE=/opt/swarvex
DATA=/opt/swarvex-data
DOWNLOADS=$DATA/downloads
BASE_HOST=$(grep ^BASE_HOST= "$CODE/infra/stack/.env" | cut -d= -f2-)
PLATFORM="https://$BASE_HOST"
VERSION=${VERSION:-0.1.$(date -u +%Y%m%d%H%M)}

[ -f "$DATA/release_key" ] || ssh-keygen -q -t ed25519 -N "" -C "swarvex-release" -f "$DATA/release_key"
chmod 600 "$DATA/release_key"

TARGET=$DOWNLOADS/$VERSION
BUILD=$(mktemp -d)
trap 'rm -rf "$BUILD"' EXIT
mkdir -p "$TARGET"
for ARCH in amd64 arm64; do
  echo "== Runner $VERSION for linux/$ARCH"
  docker run --rm -v "$CODE/runner:/src:ro" -v "$BUILD:/out" -w /src -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=$ARCH -e GOCACHE=/tmp/gc \
    golang:1 go build -trimpath -ldflags="-s -w" -o /out/swarvex .
  PACKAGE=$BUILD/package-$ARCH
  mkdir -p "$PACKAGE/sandbox"
  cp "$BUILD/swarvex" "$CODE/runner/Containerfile.proxy" "$PACKAGE/"
  cp "$CODE/runner/sandbox/Containerfile" "$PACKAGE/sandbox/"
  tar -czf "$TARGET/swarvex-$VERSION-linux-$ARCH.tar.gz" -C "$PACKAGE" .
done
( cd "$TARGET" && sha256sum swarvex-*.tar.gz > SHA256SUMS )
ssh-keygen -q -Y sign -f "$DATA/release_key" -n swarvex-release "$TARGET/SHA256SUMS"   # creates SHA256SUMS.sig

PUB=$(cut -d' ' -f1-2 "$DATA/release_key.pub")
sed -e "s#@PLATFORM@#$PLATFORM#g" -e "s#@VERSION@#$VERSION#g" -e "s#@PUBKEY@#$PUB#g" \
  "$CODE/runner/install.sh.in" > "$DOWNLOADS/install.sh"
chmod -R a+rX "$DOWNLOADS"
echo "== Release $VERSION ready: $PLATFORM/install.sh"
ls -la "$TARGET"
