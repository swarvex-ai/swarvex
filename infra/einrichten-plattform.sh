#!/bin/sh
# Richtet die Plattform auf einem frischen Debian-13-Server ein (als root ausführen).
# Mehrfach ausführbar: Vorhandenes (Geheimnisse, Schlüssel, Daten) bleibt erhalten.
#
#   BASIS_HOST=swarvex.example.org [TLS_MODUS=internal | TLS_MODUS=admin@domain] \
#   GITHUB_CLIENT_ID=… GITHUB_CLIENT_SECRET=… (oder CODEBERG_*/GITEA_*) \
#   sh infra/einrichten-plattform.sh
#
# Vorher: Code nach /opt/swarvex kopieren (git clone oder rsync). Einzelheiten: docs/selbst-hosten.md
set -eu
CODE=/opt/swarvex
DATEN=/opt/swarvex-daten
STACK=$CODE/infra/stack
# Bei Wiederholung kommen die Werte aus der vorhandenen .env.
if [ -f "$STACK/.env" ]; then BASIS_HOST=${BASIS_HOST:-$(grep ^BASIS_HOST= "$STACK/.env" | cut -d= -f2-)}; fi
: "${BASIS_HOST:?BASIS_HOST fehlt (Domain oder IP der Plattform)}"

echo "== 1. Pakete"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq curl ca-certificates docker.io docker-compose git git-lfs jq rsync openssh-client >/dev/null
systemctl enable --now docker >/dev/null

echo "== 2. Geheimnisse (.env, nur einmal erzeugt)"
if [ ! -f "$STACK/.env" ]; then
  : "${GITEA_URL:?GITEA_URL fehlt}"
  : "${GITEA_CLIENT_ID:?GITEA_CLIENT_ID fehlt (OAuth-App im Konto-Anbieter)}"
  : "${GITEA_CLIENT_SECRET:?GITEA_CLIENT_SECRET fehlt}"
  umask 077
  cat > "$STACK/.env" <<EOF
POSTGRES_PASSWORD=$(head -c 24 /dev/urandom | base64 | tr -d '/+=')
BASIS_URL=https://$BASIS_HOST
BASIS_HOST=$BASIS_HOST
TLS_MODUS=${TLS_MODUS:-internal}
GITEA_URL=$GITEA_URL
GITEA_CLIENT_ID=$GITEA_CLIENT_ID
GITEA_CLIENT_SECRET=$GITEA_CLIENT_SECRET
PRUEFSTRASSE_GEHEIMNIS=$(head -c 32 /dev/urandom | base64 | tr -d '/+=')
EOF
  echo "   .env angelegt"
else
  echo "   .env vorhanden – unverändert"
fi

echo "== 3. Schlüssel (Deploy-Keys) in $DATEN"
mkdir -p "$DATEN" /var/lib/swarvex-pruefstrasse
[ -f "$DATEN/deploy_key" ] || ssh-keygen -q -t ed25519 -N "" -C "swarvex snapshot (read-only)" -f "$DATEN/deploy_key"
[ -f "$DATEN/pruef_key" ]  || ssh-keygen -q -t ed25519 -N "" -C "swarvex pruefstrasse (read-only)" -f "$DATEN/pruef_key"
[ -f "$DATEN/merge_key" ]  || ssh-keygen -q -t ed25519 -N "" -C "swarvex merge-bot (write)" -f "$DATEN/merge_key"
for k in deploy_key pruef_key merge_key; do [ -f "$DATEN/$k.pub" ] || ssh-keygen -y -f "$DATEN/$k" > "$DATEN/$k.pub"; done
chown 1000:1000 "$DATEN/deploy_key"; chmod 600 "$DATEN/deploy_key" "$DATEN/pruef_key" "$DATEN/merge_key"

mkdir -p "$DATEN/downloads"

echo "== 4. Plattform-Stack (Postgres, API/MCP, Caddy)"
cd "$STACK"
docker compose up -d --build
for i in $(seq 1 30); do curl -sk "https://$BASIS_HOST/healthz" >/dev/null 2>&1 && break; sleep 2; done
docker compose exec -T caddy cat /data/caddy/pki/authorities/local/root.crt > "$DATEN/ca.crt" 2>/dev/null || true

echo "== 5. Prüfstraße und Merge-Bot"
cd "$CODE"
# Die Prüfstraße nutzt das Paket umgebung aus dem Runner-Modul: ganzes Repo einbinden.
docker run --rm -v "$PWD:/src" -w /src/pruefstrasse -e CGO_ENABLED=0 -e GOCACHE=/tmp/gc golang:1 go build -trimpath -ldflags="-s -w" -o swarvex-pruefstrasse .
install -m 755 pruefstrasse/swarvex-pruefstrasse /usr/local/bin/ && rm -f pruefstrasse/swarvex-pruefstrasse
GEHEIMNIS=$(grep ^PRUEFSTRASSE_GEHEIMNIS= "$STACK/.env" | cut -d= -f2-)
umask 077
cat > "$DATEN/pruefstrasse.env" <<EOF
SWARVEX_PLATTFORM=https://$BASIS_HOST
SWARVEX_PRUEFSTRASSE_GEHEIMNIS=$GEHEIMNIS
SWARVEX_CA=$DATEN/ca.crt
SWARVEX_ARBEIT=/var/lib/swarvex-pruefstrasse
SWARVEX_DEPLOY_KEY_LESEN=$DATEN/pruef_key
SWARVEX_DEPLOY_KEY_SCHREIBEN=$DATEN/merge_key
EOF
install -m 644 "$CODE/infra/systemd/"swarvex-*.service "$CODE/infra/systemd/"swarvex-*.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now swarvex-pruefen.timer swarvex-mergen.timer >/dev/null

echo "== 6. Runner-Release + Einzeiler"
sh "$CODE/infra/release.sh" | tail -2

echo
echo "== Fertig. Noch von Hand (docs/selbst-hosten.md, Abschnitt 3):"
echo "   Deploy-Keys im Projekt-Repo eintragen:"
echo "     lesend    (Snapshot):    $(cat "$DATEN/deploy_key.pub")"
echo "     lesend    (Prüfstraße):  $(cat "$DATEN/pruef_key.pub")"
echo "     SCHREIBEND (Merge-Bot):  $(cat "$DATEN/merge_key.pub")"
echo "   CA für Clients (nur bei interner CA): $DATEN/ca.crt"
