#!/bin/sh
# Read access for Grafana to the analytics views (schema analytik, migration 010). Idempotent; deploy.sh runs it.
# The password exists only on the server (/opt/swarvex-data/analytics_db) and goes to PostgreSQL via stdin.
# Run as root on the platform:  sh /opt/swarvex/infra/analytics-setup.sh
set -eu
DATA=/opt/swarvex-data
DATEI=$DATA/analytics_db
cd /opt/swarvex/infra/stack
[ -s "$DATEI" ] || { head -c 24 /dev/urandom | base64 | tr -d '/+=' > "$DATEI"; }
chmod 644 "$DATEI"   # read by the Grafana container user
# The role comes from the API's migration; wait until it exists
for i in $(seq 1 60); do
  docker compose exec -T postgres psql -U swarvex -d swarvex -Atc "select 1 from pg_roles where rolname = 'analytik_leser'" 2>/dev/null | grep -q 1 && break
  sleep 2
done
printf "alter role analytik_leser login password '%s' connection limit 5;\n" "$(cat "$DATEI")" \
  | docker compose exec -T postgres psql -U swarvex -d swarvex -q -v ON_ERROR_STOP=1 >/dev/null
echo "analytics read access ready"
