#!/bin/sh
# Grafana with two areas and GitHub sign-in (docs/self-hosting.md, "Analytics"). Idempotent; deploy.sh runs it.
#
#   sh infra/grafana-setup.sh files   before Grafana starts: settings, secret files, Caddy site
#   sh infra/grafana-setup.sh areas   after Grafana started: area "Marketing" with its data source and dashboards
#
# Area "Betrieb" (org 1): everything, for team @swarvex-ai/admins only: server, security, leads and moderation,
#   marketing and visitors (one place, no switching; infra/monitoring/grafana/provisioning/dashboards).
# Area "Marketing" (org 2): only the analytics views with the dashboards Marketing and Visitors; @swarvex-ai/marketing
#   as viewer. Marketing therefore cannot query Prometheus or Loki, not even through the API.
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
  # Bis 31.10.2026 lag das Dashboard im Datei-Provisioning (Bereich „Main“); deploy.sh löscht unter infra nichts
  rm -f "$CODE/infra/monitoring/grafana/dashboards/analytik.json"
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
GF_AUTH_GITHUB_ORG_MAPPING=@swarvex-ai/admins:1:Admin,@swarvex-ai/marketing:2:Viewer
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
# Bereich 1 heißt „Betrieb“ (Grafana nennt ihn „Main Org.“); die GitHub-Zuordnung nutzt die Nummer 1, nicht den Namen
curl -fs -u "$A" -H 'content-type: application/json' -X PUT $G/api/orgs/1 -d '{"name":"Betrieb"}' >/dev/null
# Startseite im Bereich „Betrieb“: das Server-Dashboard statt der Grafana-Willkommensseite
curl -fs -u "$A" -H 'X-Grafana-Org-Id: 1' -H 'content-type: application/json' -X PUT $G/api/org/preferences -d '{"homeDashboardUID":"swarvex-server","timezone":"Europe/Berlin"}' >/dev/null
# Bereich 2 heißt „Marketing“ (bis 5.11.2026 „Auswertung“); die GitHub-Zuordnung nutzt die Nummer 2
if [ "$(curl -s -o /dev/null -w '%{http_code}' -u "$A" $G/api/orgs/2)" != 200 ]; then
  curl -fs -u "$A" -H 'content-type: application/json' -X POST $G/api/orgs -d '{"name":"Marketing"}' >/dev/null
fi
curl -fs -u "$A" -H 'content-type: application/json' -X PUT $G/api/orgs/2 -d '{"name":"Marketing"}' >/dev/null
org=2
H="X-Grafana-Org-Id: $org"
quelle=$(printf '{"name":"Analytik","uid":"analytik","type":"grafana-postgresql-datasource","access":"proxy","url":"postgres:5432","user":"analytik_leser","isDefault":true,"jsonData":{"database":"swarvex","sslmode":"disable","postgresVersion":1500,"timeInterval":"1h"},"secureJsonData":{"password":"%s"}}' "$(cat /run/secrets/analytics_db)")
# Bereich „Betrieb“ hat dieselbe Quelle aus dem Datei-Provisioning (provisioning/datasources)
if [ "$(curl -s -o /dev/null -w '%{http_code}' -u "$A" -H "$H" $G/api/datasources/uid/analytik)" = 200 ]; then
  printf '%s' "$quelle" | curl -fs -u "$A" -H "$H" -H 'content-type: application/json' -X PUT $G/api/datasources/uid/analytik -d @- >/dev/null
else
  printf '%s' "$quelle" | curl -fs -u "$A" -H "$H" -H 'content-type: application/json' -X POST $G/api/datasources -d @- >/dev/null
fi
# Bis 5.11.2026 hieß der Ordner der Betriebs-Dashboards „swarvex“: leer zurückgeblieben, also weg
alt=$(curl -fs -u "$A" -H 'X-Grafana-Org-Id: 1' "$G/api/folders" | tr '{' '\n' | grep '"title":"swarvex"' | grep -oE '"uid":"[^"]*"' | cut -d'"' -f4)
if [ -n "$alt" ] && ! curl -fs -u "$A" -H 'X-Grafana-Org-Id: 1' "$G/api/search?folderUIDs=$alt" | grep -q '"uid"'; then
  curl -fs -u "$A" -H 'X-Grafana-Org-Id: 1' -X DELETE "$G/api/folders/$alt" >/dev/null || true
fi
# Admins gehören nur noch in „Betrieb“ (dort sehen sie alles); Grafana gleicht das sonst erst bei der nächsten Anmeldung ab
for u in $(curl -fs -u "$A" "$G/api/orgs/1/users" | tr '{' '\n' | grep '"role":"Admin"' | grep -v '"login":"admin"' | grep -oE '"userId":[0-9]+' | cut -d: -f2); do
  curl -fs -u "$A" -X DELETE "$G/api/orgs/2/users/$u" >/dev/null 2>&1 || true
done
# Every dashboard of the area (analytik.json: usage and swarm, website.json: visitors of the website)
for d in /etc/grafana/analytik/*.json; do
  { printf '{"overwrite":true,"dashboard":'; cat "$d"; printf '}'; } \
    | curl -fs -u "$A" -H "$H" -H 'content-type: application/json' -X POST $G/api/dashboards/db -d @- >/dev/null
done
curl -fs -u "$A" -H "$H" -H 'content-type: application/json' -X PUT $G/api/org/preferences -d '{"homeDashboardUID":"swarvex-analytik","timezone":"Europe/Berlin"}' >/dev/null
echo "grafana: areas Betrieb (all dashboards) and Marketing ready"
EOF
}

case "${1:-}" in
  files) dateien ;;
  areas) bereiche ;;
  *) echo "usage: grafana-setup.sh files|areas"; exit 2 ;;
esac
