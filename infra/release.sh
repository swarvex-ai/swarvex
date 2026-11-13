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
# Version: runner/VERSION (semantic: patch for fixes, minor for new features). A version is published once;
# if the runner's code changed since, the release stops until runner/VERSION is raised.
# Live signs on the visionary's machine (hardware key), never on this server: infra/release-local.sh
if grep -q '^RELEASE_SIGNING=local' "$CODE/infra/stack/.env"; then
  echo "== Releases of this platform are signed on the visionary's machine: sh infra/release-local.sh <host> (nothing done here)"
  exit 0
fi
VERSION=${VERSION:-$(tr -d ' \n' < "$CODE/runner/VERSION")}
echo "$VERSION" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' || { echo "runner/VERSION must look like 0.9.0, not '$VERSION'"; exit 1; }
# What's new (swarvex update shows the entries since the installed version); kept current even without a new version
mkdir -p "$DOWNLOADS" && cp "$CODE/runner/CHANGELOG.md" "$DOWNLOADS/CHANGELOG.md" && chmod a+r "$DOWNLOADS/CHANGELOG.md"
QUELLE=$(cd "$CODE" && find runner -type f ! -name '*_test.go' ! -name '*.md' | LC_ALL=C sort | xargs sha256sum | sha256sum | cut -d' ' -f1)
if [ -f "$DOWNLOADS/$VERSION/SOURCE" ]; then
  if [ "$(cat "$DOWNLOADS/$VERSION/SOURCE")" = "$QUELLE" ]; then
    echo "== Release v$VERSION is already published with this code; nothing to do"
    exit 0
  fi
  echo "The runner changed since v$VERSION was published: raise runner/VERSION (e.g. to the next patch version) and deploy again."
  exit 1
fi

[ -f "$DATA/release_key" ] || ssh-keygen -q -t ed25519 -N "" -C "swarvex-release" -f "$DATA/release_key"
chmod 600 "$DATA/release_key"

TARGET=$DOWNLOADS/$VERSION
BUILD=$(mktemp -d)
trap 'rm -rf "$BUILD"' EXIT
mkdir -p "$TARGET"
for ARCH in amd64 arm64; do
  echo "== Runner v$VERSION for linux/$ARCH"
  docker run --rm --pull=always -v "$CODE/runner:/src:ro" -v "$BUILD:/out" -w /src -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=$ARCH -e GOCACHE=/tmp/gc -e VERSION="$VERSION" \
    golang:1 sh -c 'go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /out/swarvex . && cp /etc/ssl/certs/ca-certificates.crt /out/'
  # The key proxy image is empty (FROM scratch): without this CA list it cannot reach any HTTPS model provider.
  PACKAGE=$BUILD/package-$ARCH
  mkdir -p "$PACKAGE/sandbox"
  cp "$BUILD/swarvex" "$BUILD/ca-certificates.crt" "$CODE/runner/Containerfile.proxy" "$PACKAGE/"
  cp "$CODE/runner/sandbox/Containerfile" "$PACKAGE/sandbox/"
  tar -czf "$TARGET/swarvex-$VERSION-linux-$ARCH.tar.gz" -C "$PACKAGE" .
done
PUB=$(cut -d' ' -f1-2 "$DATA/release_key.pub")
sed -e "s#@PLATFORM@#$PLATFORM#g" -e "s#@VERSION@#$VERSION#g" -e "s#@PUBKEY@#$PUB#g" -e "s#@ROTATION_SIG@##g" \
  "$CODE/runner/install.sh.in" > "$TARGET/install.sh"
# install.sh is signed with the packages: `swarvex update` runs it only when the signed SHA256SUMS lists it
( cd "$TARGET" && sha256sum swarvex-*.tar.gz install.sh > SHA256SUMS )
echo "$QUELLE" > "$TARGET/SOURCE"
ssh-keygen -q -Y sign -f "$DATA/release_key" -n swarvex-release "$TARGET/SHA256SUMS"   # creates SHA256SUMS.sig
cp "$TARGET/install.sh" "$DOWNLOADS/install.sh"
# Windows: sets up the WSL distribution and runs install.sh inside it (signature check happens there)
sed -e "s#@PLATFORM@#$PLATFORM#g" "$CODE/runner/install.ps1.in" > "$DOWNLOADS/install.ps1"
chmod -R a+rX "$DOWNLOADS"
echo "== Release v$VERSION ready: $PLATFORM/install.sh"
ls -la "$TARGET"
