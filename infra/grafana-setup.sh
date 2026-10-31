#!/bin/sh
# Grafana with two areas and GitHub sign-in (docs/self-hosting.md, "Analytics"). Idempotent; deploy.sh runs it.
#
#   sh infra/grafana-setup.sh files   before Grafana starts: settings, secret files, Caddy site
#   sh infra/grafana-setup.sh areas   after Grafana started: area "Auswertung" with its data source and dashboard
#
# Area "Main" (org 1, "Betrieb"): server metrics and logs, only team @swarvex-ai/admins.
# Area "Auswertung" (org 2): only the analytics views; @swarvex-ai/marketing as viewer, admins as admin.
# Marketing therefore cannot query Prometheus or Loki, not even through the API.
#
# Inputs on the server (not in the repository):
#   /opt/swarvex/infra/stack/.env      ANALYTICS_HOST=analytics.<domain>   (live only; the lab stays on the SSH tunnel)
#   /opt/swarvex-data/grafana-oauth.env  GRAFANA_GITHUB_CLIENT_ID=… GRAFANA_GITHUB_CLIENT_SECRET=…   (GitHub OAuth app)
# Without both, Grafana stays as before: admin password, reachable only through the SSH tunnel.
set -eu
DATA=/opt/swarvex-data
CODE=/opt/swarvex

dateien() {
  umask 022
  host=$(sed -n 's/^ANALYTICS_HOST=//p' "$CODE/infra/stack/.env" 2>/dev/null | tr -d '"' | tail -1)
  id=$(sed -n 's/^GRAFANA_GITHUB_CLIENT_ID=//p' "$DATA/grafana-oauth.env" 2>/dev/null | tr -d '"' | tail -1)
  geheim=$(sed -n 's/^GRAFANA_GITHUB_CLIENT_SECRET=//p' "$DATA/grafana-oauth.env" 2>/dev/null | tr -d '"' | tail -1)
  printf '%s' "$geheim" > "$DATA/grafana_github_secret"; chmod 644 "$DATA/grafana_github_secret"   # read by the Grafana container user
  if [ -n "$host" ] && [ -n "$id" ] && [ -n "$geheim" ]; then
    cat > "$DATA/grafana.env" <<EOF
GF_SERVER_ROOT_URL=https://$host/
GF_SECURITY_COOKIE_SECURE=true
GF_AUTH_DISABLE_LOGIN_FORM=true
GF_AUTH_OAUTH_AUTO_LOGIN=true
GF_AUTH_GITHUB_ENABLED=true
GF_AUTH_GITHUB_CLIENT_ID=$id
GF_AUTH_GITHUB_CLIENT_SECRET__FILE=/run/secrets/grafana_github_secret
GF_AUTH_GITHUB_SCOPES=user:email,read:org
GF_AUTH_GITHUB_ALLOWED_ORGANIZATIONS=swarvex-ai
GF_AUTH_GITHUB_ALLOW_SIGN_UP=true
GF_AUTH_GITHUB_ORG_MAPPING=@swarvex-ai/admins:1:Admin,@swarvex-ai/admins:Auswertung:Admin,@swarvex-ai/marketing:Auswertung:Viewer
GF_AUTH_GITHUB_ROLE_ATTRIBUTE_STRICT=true
GF_USERS_AUTO_ASSIGN_ORG_ID=2
EOF
    cat > "$DATA/sites/analytics.caddy" <<EOF
# Grafana (infra/grafana-setup.sh): sign-in only via GitHub; no password logins from outside
$host {
	tls $(sed -n 's/^TLS_MODE=//p' "$CODE/infra/stack/.env" | tail -1)
	header X-Robots-Tag "noindex, nofollow"
	header Strict-Transport-Security "max-age=31536000"
	@passwort header Authorization "Basic *"
	respond @passwort 403
	reverse_proxy grafana:3000
}
EOF
    echo "grafana: GitHub sign-in on https://$host"
  else
    : > "$DATA/grafana.env"
    rm -f "$DATA/sites/analytics.caddy"
    echo "grafana: SSH tunnel only (ANALYTICS_HOST or grafana-oauth.env missing)"
  fi
}

bereiche() {
  cd "$CODE/infra/monitoring"
  # Inside the Grafana container: admin password and analytics password come from its mounted secret files
  docker compose exec -T grafana sh -s <<'EOF'
set -eu
G=http://localhost:3000
A="admin:$(cat /run/secrets/grafana_admin)"
for i in $(seq 1 60); do curl -fs $G/api/health >/dev/null 2>&1 && break; sleep 2; done
code=$(curl -s -o /dev/null -w '%{http_code}' -u "$A" $G/api/orgs/name/Auswertung)
[ "$code" = 200 ] || curl -fs -u "$A" -H 'content-type: application/json' -X POST $G/api/orgs -d '{"name":"Auswertung"}' >/dev/null
org=$(curl -fs -u "$A" $G/api/orgs/name/Auswertung | sed 's/.*"id":\([0-9]*\).*/\1/')
[ "$org" = 2 ] || { echo "area Auswertung has id $org, expected 2 (GF_USERS_AUTO_ASSIGN_ORG_ID)"; exit 1; }
H="X-Grafana-Org-Id: $org"
quelle=$(printf '{"name":"Analytik","uid":"analytik","type":"grafana-postgresql-datasource","access":"proxy","url":"postgres:5432","user":"analytik_leser","isDefault":true,"jsonData":{"database":"swarvex","sslmode":"disable","postgresVersion":1500,"timeInterval":"1h"},"secureJsonData":{"password":"%s"}}' "$(cat /run/secrets/analytics_db)")
if [ "$(curl -s -o /dev/null -w '%{http_code}' -u "$A" -H "$H" $G/api/datasources/uid/analytik)" = 200 ]; then
  printf '%s' "$quelle" | curl -fs -u "$A" -H "$H" -H 'content-type: application/json' -X PUT $G/api/datasources/uid/analytik -d @- >/dev/null
else
  printf '%s' "$quelle" | curl -fs -u "$A" -H "$H" -H 'content-type: application/json' -X POST $G/api/datasources -d @- >/dev/null
fi
{ printf '{"overwrite":true,"dashboard":'; cat /etc/grafana/analytik/analytik.json; printf '}'; } \
  | curl -fs -u "$A" -H "$H" -H 'content-type: application/json' -X POST $G/api/dashboards/db -d @- >/dev/null
curl -fs -u "$A" -H "$H" -H 'content-type: application/json' -X PUT $G/api/org/preferences -d '{"homeDashboardUID":"swarvex-analytik","timezone":"Europe/Berlin"}' >/dev/null
echo "grafana: area Auswertung ready (org $org)"
EOF
}

case "${1:-}" in
  files) dateien ;;
  areas) bereiche ;;
  *) echo "usage: grafana-setup.sh files|areas"; exit 2 ;;
esac
