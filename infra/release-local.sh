#!/bin/sh
# Builds and signs a runner release on the visionary's machine and uploads it to a platform. The signing key lives
# on a hardware key (Nitrokey, YubiKey): every release needs one touch, and the server never holds the key.
#
#   sh infra/release-local.sh swarvex-live          (or schwarm-plattform for the lab)
#
# Needs: podman or docker, ssh access to the platform (like infra/deploy.sh), and the key created once with
#   ssh-keygen -t ed25519-sk -O application=ssh:swarvex-release -C swarvex-release -N "" -f ~/.config/swarvex-release/release_sk
# RELEASE_KEY=<file> uses another key file (for tests in the lab with a software key).
#
# Key change: when the platform's current install.sh names a different key, the new key is signed once with the
# old one (on the server, where the old key still is) and that signature goes into install.sh. Runners from 0.9.3 on
# accept the new key only with it (runner/update.go, schluesselwechselGeprueft). After all runners have moved,
# delete the old key on the server: rm /opt/swarvex-data/release_key*
set -eu
TARGET=${1:?usage: sh infra/release-local.sh <ssh host of the platform>}
REPO=$(cd "$(dirname "$0")/.." && pwd)
KEY=${RELEASE_KEY:-$HOME/.config/swarvex-release/release_sk}
[ -f "$KEY" ] && [ -f "$KEY.pub" ] || { echo "No signing key at $KEY (see the comment at the top of this script)"; exit 1; }
NEU=$(cut -d' ' -f1-2 "$KEY.pub")
RUN=$(command -v podman || command -v docker) || { echo "needs podman or docker"; exit 1; }
DATA=/opt/swarvex-data
DOWNLOADS=$DATA/downloads

VERSION=$(tr -d ' \n' < "$REPO/runner/VERSION")
echo "$VERSION" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' || { echo "runner/VERSION must look like 0.9.0, not '$VERSION'"; exit 1; }
# The same fingerprint of the runner's source as infra/release.sh: a version is published once
QUELLE=$(cd "$REPO" && find runner -type f ! -name '*_test.go' ! -name '*.md' | LC_ALL=C sort | xargs sha256sum | sha256sum | cut -d' ' -f1)
VORHANDEN=$(ssh "$TARGET" "cat $DOWNLOADS/$VERSION/SOURCE 2>/dev/null || true")
if [ "$VORHANDEN" = "$QUELLE" ]; then echo "== v$VERSION is already published with this code; nothing to do"; exit 0; fi
[ -z "$VORHANDEN" ] || { echo "v$VERSION was published with other code: raise runner/VERSION"; exit 1; }
BASE_HOST=$(ssh "$TARGET" "grep ^BASE_HOST= /opt/swarvex/infra/stack/.env | cut -d= -f2-")
PLATFORM="https://$BASE_HOST"

BUILD=$(mktemp -d)
trap 'rm -rf "$BUILD"' EXIT
mkdir -p "$BUILD/out" "$BUILD/$VERSION"
for ARCH in amd64 arm64; do
  echo "== Runner v$VERSION for linux/$ARCH (local build)"
  # --pull=always: every release gets the newest Go patch release (13 Nov 2026: the cached image still had go1.27.3,
  # GO-2026-6859 is fixed in go1.27.4)
  "$RUN" run --rm --pull=always -v "$REPO/runner:/src:ro,Z" -v "$BUILD/out:/out:Z" -w /src -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=$ARCH -e GOCACHE=/tmp/gc \
    -e VERSION="$VERSION" docker.io/library/golang:1 \
    sh -c 'go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /out/swarvex . && cp /etc/ssl/certs/ca-certificates.crt /out/'
  P="$BUILD/package-$ARCH"
  mkdir -p "$P/sandbox"
  cp "$BUILD/out/swarvex" "$BUILD/out/ca-certificates.crt" "$REPO/runner/Containerfile.proxy" "$P/"
  cp "$REPO/runner/sandbox/Containerfile" "$P/sandbox/"
  tar -czf "$BUILD/$VERSION/swarvex-$VERSION-linux-$ARCH.tar.gz" -C "$P" .
done
# Key change: the key the platform signs with today, and if it differs, the new key signed by it (once)
ALT=$(ssh "$TARGET" "sed -n 's/^SIGNING_KEY=\"\\(.*\\)\"$/\\1/p' $DOWNLOADS/install.sh 2>/dev/null || true")
ROT=""
if [ -n "$ALT" ] && [ "$ALT" != "$NEU" ]; then
  if ssh "$TARGET" "test -f $DOWNLOADS/key-rotation.sig && grep -qxF '$NEU' $DOWNLOADS/key-rotation.pub"; then
    ssh "$TARGET" "cat $DOWNLOADS/key-rotation.sig" > "$BUILD/rotation.sig"
  else
    echo "== Key change: the platform's current key signs the new one (on $TARGET)"
    ssh "$TARGET" "test -f $DATA/release_key" || { echo "the old key is no longer on $TARGET; runners must be installed again with the one-liner"; exit 1; }
    printf '%s' "$NEU" | ssh "$TARGET" "ssh-keygen -q -Y sign -f $DATA/release_key -n swarvex-key-rotation" > "$BUILD/rotation.sig"
    printf '%s\n' "$NEU" > "$BUILD/rotation.pub"
    scp -q "$BUILD/rotation.sig" "$TARGET:$DOWNLOADS/key-rotation.sig"
    scp -q "$BUILD/rotation.pub" "$TARGET:$DOWNLOADS/key-rotation.pub"
  fi
  printf 'swarvex-release %s\n' "$ALT" > "$BUILD/allowed-alt"
  printf '%s' "$NEU" | ssh-keygen -Y verify -f "$BUILD/allowed-alt" -I swarvex-release -n swarvex-key-rotation -s "$BUILD/rotation.sig" >/dev/null \
    || { echo "the key change signature does not verify; nothing was uploaded"; exit 1; }
  ROT=$(base64 -w0 < "$BUILD/rotation.sig")
elif [ "$ALT" = "$NEU" ] && ssh "$TARGET" "test -f $DOWNLOADS/key-rotation.sig"; then
  ROT=$(ssh "$TARGET" "base64 -w0 < $DOWNLOADS/key-rotation.sig")   # keep it until every runner has moved
fi

sed -e "s#@PLATFORM@#$PLATFORM#g" -e "s#@VERSION@#$VERSION#g" -e "s#@PUBKEY@#$NEU#g" -e "s#@ROTATION_SIG@#$ROT#g" \
  "$REPO/runner/install.sh.in" > "$BUILD/$VERSION/install.sh"
sed -e "s#@PLATFORM@#$PLATFORM#g" "$REPO/runner/install.ps1.in" > "$BUILD/install.ps1"
sh -n "$BUILD/$VERSION/install.sh"
# The one-liner copy at /install.sh is the same file: `swarvex update` runs it only when the signed SHA256SUMS of
# its version lists it (runner/update.go, releaseGeprueft; security audit of 12 Nov 2026)
cp "$BUILD/$VERSION/install.sh" "$BUILD/install.sh"
( cd "$BUILD/$VERSION" && sha256sum swarvex-*.tar.gz install.sh > SHA256SUMS )
echo "$QUELLE" > "$BUILD/$VERSION/SOURCE"
echo "== Signing the checksums: touch your key when it blinks"
ssh-keygen -q -Y sign -f "$KEY" -n swarvex-release "$BUILD/$VERSION/SHA256SUMS"
printf 'swarvex-release %s\n' "$NEU" > "$BUILD/allowed"
ssh-keygen -Y verify -f "$BUILD/allowed" -I swarvex-release -n swarvex-release -s "$BUILD/$VERSION/SHA256SUMS.sig" < "$BUILD/$VERSION/SHA256SUMS" >/dev/null

# `swarvex update` shows what's new from this file: the version released now carries today's date, not "(unreleased)"
sed "s/^## $VERSION (unreleased)\$/## $VERSION ($(LC_ALL=C date '+%-d %b %Y'))/" "$REPO/runner/CHANGELOG.md" > "$BUILD/CHANGELOG.md"

echo "== Uploading to $TARGET"
ssh "$TARGET" "mkdir -p $DOWNLOADS/$VERSION"
scp -q "$BUILD/$VERSION/"* "$TARGET:$DOWNLOADS/$VERSION/"
scp -q "$BUILD/install.sh" "$BUILD/install.ps1" "$BUILD/CHANGELOG.md" "$TARGET:$DOWNLOADS/"
ssh "$TARGET" "chmod -R a+rX $DOWNLOADS"
echo "== Release v$VERSION ready: $PLATFORM/install.sh (signed with $(echo "$NEU" | cut -c1-30)…)"
