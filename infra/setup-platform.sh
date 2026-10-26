#!/bin/sh
# Sets up the platform on a fresh Debian 13 server (run as root).
# Safe to run again: existing secrets, keys and data are kept.
#
#   BASE_HOST=swarvex.example.org [TLS_MODE=internal | TLS_MODE=admin@domain] \
#   GITHUB_CLIENT_ID=… GITHUB_CLIENT_SECRET=… (or CODEBERG_*/GITEA_*) \
#   sh infra/setup-platform.sh
#
# Before: copy the code to /opt/swarvex (git clone or rsync). Details: docs/self-hosting.md
set -eu
CODE=/opt/swarvex
DATA=/opt/swarvex-data
STACK=$CODE/infra/stack
# When run again, the values come from the existing .env.
if [ -f "$STACK/.env" ]; then BASE_HOST=${BASE_HOST:-$(grep ^BASE_HOST= "$STACK/.env" | cut -d= -f2-)}; fi
: "${BASE_HOST:?BASE_HOST is missing (domain or IP of the platform)}"

echo "== 1. Packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq curl ca-certificates docker.io docker-compose git git-lfs jq rsync openssh-client >/dev/null
systemctl enable --now docker >/dev/null

echo "== 2. Secrets (.env, created only once)"
if [ ! -f "$STACK/.env" ]; then
  umask 077
  cat > "$STACK/.env" <<EOF
POSTGRES_PASSWORD=$(head -c 24 /dev/urandom | base64 | tr -d '/+=')
BASE_URL=https://$BASE_HOST
BASE_HOST=$BASE_HOST
TLS_MODE=${TLS_MODE:-internal}
GITHUB_CLIENT_ID=${GITHUB_CLIENT_ID:-}
GITHUB_CLIENT_SECRET=${GITHUB_CLIENT_SECRET:-}
CODEBERG_CLIENT_ID=${CODEBERG_CLIENT_ID:-}
CODEBERG_CLIENT_SECRET=${CODEBERG_CLIENT_SECRET:-}
GITEA_URL=${GITEA_URL:-}
GITEA_CLIENT_ID=${GITEA_CLIENT_ID:-}
GITEA_CLIENT_SECRET=${GITEA_CLIENT_SECRET:-}
ACCOUNT_MIN_AGE_DAYS=30
PIPELINE_SECRET=$(head -c 32 /dev/urandom | base64 | tr -d '/+=')
EOF
  echo "   .env created"
else
  echo "   .env exists, unchanged"
fi

echo "== 3. Deploy keys in $DATA"
mkdir -p "$DATA" /var/lib/swarvex-pipeline
[ -f "$DATA/deploy_key" ] || ssh-keygen -q -t ed25519 -N "" -C "swarvex snapshot (read-only)" -f "$DATA/deploy_key"
[ -f "$DATA/check_key" ]  || ssh-keygen -q -t ed25519 -N "" -C "swarvex test pipeline (read-only)" -f "$DATA/check_key"
[ -f "$DATA/merge_key" ]  || ssh-keygen -q -t ed25519 -N "" -C "swarvex merge bot (write)" -f "$DATA/merge_key"
for k in deploy_key check_key merge_key; do [ -f "$DATA/$k.pub" ] || ssh-keygen -y -f "$DATA/$k" > "$DATA/$k.pub"; done
chown 1000:1000 "$DATA/deploy_key"; chmod 600 "$DATA/deploy_key" "$DATA/check_key" "$DATA/merge_key"

mkdir -p "$DATA/downloads"

echo "== 4. Platform stack (Postgres, API/MCP, website, Caddy)"
cd "$STACK"
docker compose up -d --build
for i in $(seq 1 30); do curl -sk "https://$BASE_HOST/healthz" >/dev/null 2>&1 && break; sleep 2; done
docker compose exec -T caddy cat /data/caddy/pki/authorities/local/root.crt > "$DATA/ca.crt" 2>/dev/null || true

echo "== 5. Test pipeline and merge bot"
cd "$CODE"
# The pipeline uses the environment package from the runner module: mount the whole repo.
docker run --rm -v "$PWD:/src" -w /src/pipeline -e CGO_ENABLED=0 -e GOCACHE=/tmp/gc golang:1 go build -trimpath -ldflags="-s -w" -o swarvex-pipeline .
install -m 755 pipeline/swarvex-pipeline /usr/local/bin/ && rm -f pipeline/swarvex-pipeline
SECRET=$(grep ^PIPELINE_SECRET= "$STACK/.env" | cut -d= -f2-)
umask 077
cat > "$DATA/pipeline.env" <<EOF
SWARVEX_PLATFORM=https://$BASE_HOST
SWARVEX_PIPELINE_SECRET=$SECRET
SWARVEX_CA=$DATA/ca.crt
SWARVEX_WORKDIR=/var/lib/swarvex-pipeline
SWARVEX_DEPLOY_KEY_READ=$DATA/check_key
SWARVEX_DEPLOY_KEY_WRITE=$DATA/merge_key
EOF
install -m 644 "$CODE/infra/systemd/"swarvex-*.service "$CODE/infra/systemd/"swarvex-*.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now swarvex-check.timer swarvex-merge.timer >/dev/null

echo "== 6. Runner release and one-line installer"
sh "$CODE/infra/release.sh" | tail -2

echo
echo "== Done. Still to do by hand (docs/self-hosting.md, section 3):"
echo "   Add the deploy keys to the project repository:"
echo "     read-only (snapshots):     $(cat "$DATA/deploy_key.pub")"
echo "     read-only (test pipeline): $(cat "$DATA/check_key.pub")"
echo "     WRITE     (merge bot):     $(cat "$DATA/merge_key.pub")"
echo "   CA for clients (internal CA only): $DATA/ca.crt"
