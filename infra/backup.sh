#!/bin/sh
# Nightly backup of the platform with restic: a PostgreSQL dump plus the server-only files
# (.env, deploy and release keys, sites). Encrypted before it leaves the server.
# Run as root on the platform (systemd timer swarvex-backup.timer):  sh /opt/swarvex/infra/backup.sh
#
# Configuration in /opt/swarvex-data/backup.env (created by setup-platform.sh, not in the repository):
#   RESTIC_REPOSITORY=sftp:<user>@<host>:swarvex    (second provider; a local path only as a stopgap)
#   RESTIC_PASSWORD_FILE=/opt/swarvex-data/backup.pass
# Keep a copy of backup.pass outside the server: without it the backups cannot be read.
# Not backed up because it can be rebuilt: downloads (infra/release.sh), certificates, the repo mirror.
set -eu
DATA=/opt/swarvex-data
STACK=/opt/swarvex/infra/stack
DUMPS=/var/backups/swarvex

[ -f "$DATA/backup.env" ] || { echo "no $DATA/backup.env, backup not configured"; exit 0; }
set -a; . "$DATA/backup.env"; set +a
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
restic forget --quiet --tag swarvex --keep-daily 7 --keep-weekly 4 --keep-monthly 6 --prune
# Sundays: read back a sample of the stored data, not only the index
if [ "$(date +%u)" = 7 ]; then restic check --read-data-subset=5%; else restic check; fi
restic snapshots --tag swarvex --latest 1
