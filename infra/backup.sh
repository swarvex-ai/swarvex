#!/bin/sh
# Nightly backup of the platform with restic: a PostgreSQL dump plus the server-only files
# (.env, deploy and release keys, sites). Encrypted before it leaves the server.
# Run as root on the platform (systemd timer swarvex-backup.timer):  sh /opt/swarvex/infra/backup.sh
#
# Configuration in /opt/swarvex-data/backup.env (created by setup-platform.sh, not in the repository):
#   RESTIC_REPOSITORY=sftp:<user>@<host>:swarvex  or  rest:https://<host>/<repo>/   (offsite; a local path only as a stopgap)
#   RESTIC_PASSWORD_FILE=/opt/swarvex-data/backup.pass
#   rest-server only:  RESTIC_REST_USERNAME=<user>  RESTIC_REST_PASSWORD_FILE=/opt/swarvex-data/<file>
#   BACKUP_APPEND_ONLY=1   the target refuses deletes (rest-server --append-only, recommended): the server cannot
#                          remove old backups, so it does not prune them either; that happens on the target's side
#   BACKUP_PING_URL=<url>  optional, called after every successful run (dead man's switch, e.g. a push monitor)
# Keep a copy of backup.pass outside the server: without it the backups cannot be read.
# Not backed up because it can be rebuilt: downloads (infra/release.sh), certificates, the repo mirror.
set -eu
DATA=/opt/swarvex-data
STACK=/opt/swarvex/infra/stack
DUMPS=/var/backups/swarvex

[ -f "$DATA/backup.env" ] || { echo "no $DATA/backup.env, backup not configured"; exit 0; }
set -a; . "$DATA/backup.env"; set +a
# restic takes the rest-server password only from the environment; it stays in a file like the repository password
if [ -n "${RESTIC_REST_PASSWORD_FILE:-}" ]; then RESTIC_REST_PASSWORD=$(cat "$RESTIC_REST_PASSWORD_FILE"); export RESTIC_REST_PASSWORD; fi
[ -n "${RESTIC_REPOSITORY:-}" ] || { echo "RESTIC_REPOSITORY is empty, backup not configured"; exit 0; }

umask 077
mkdir -p "$DUMPS"
echo "== PostgreSQL dump"
cd "$STACK"
docker compose exec -T postgres pg_dump -U swarvex -d swarvex -Fc > "$DUMPS/swarvex.dump.tmp"
mv "$DUMPS/swarvex.dump.tmp" "$DUMPS/swarvex.dump"

echo "== restic → $RESTIC_REPOSITORY"
restic cat config >/dev/null 2>&1 || restic init
restic backup --quiet --tag swarvex "$DUMPS" "$DATA" "$STACK/.env" --exclude "$DATA/downloads"
if [ "${BACKUP_APPEND_ONLY:-0}" = 1 ]; then
  echo "append-only target: old snapshots stay, pruning happens on the target"
else
  restic forget --quiet --tag swarvex --keep-daily 7 --keep-weekly 4 --keep-monthly 6 --prune
fi
# Sundays: read back a sample of the stored data, not only the index
if [ "$(date +%u)" = 7 ]; then restic check --read-data-subset=5%; else restic check; fi
restic snapshots --tag swarvex --latest 1
# Für das Sicherheits-Dashboard (Alloy liest das Verzeichnis als Textfile-Kennzahlen)
mkdir -p /var/lib/swarvex-metrics
printf '# HELP swarvex_backup_last_success_timestamp_seconds Last successful backup (infra/backup.sh)\n# TYPE swarvex_backup_last_success_timestamp_seconds gauge\nswarvex_backup_last_success_timestamp_seconds %s\n' "$(date +%s)" > /var/lib/swarvex-metrics/backup.prom.tmp
mv /var/lib/swarvex-metrics/backup.prom.tmp /var/lib/swarvex-metrics/backup.prom
if [ -n "${BACKUP_PING_URL:-}" ]; then curl -fsS -m 10 --retry 3 -o /dev/null "$BACKUP_PING_URL" || echo "ping failed"; fi
