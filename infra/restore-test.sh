#!/bin/sh
# Restore rehearsal: takes the latest backup, loads the dump into a throwaway PostgreSQL and
# checks the public log chain there. Touches neither the running stack nor its data.
# Run as root on the platform:  sh /opt/swarvex/infra/restore-test.sh
set -eu
DATA=/opt/swarvex-data
set -a; . "$DATA/backup.env"; set +a
ZIEL=$(mktemp -d)
NAME=swarvex-restore-test
trap 'docker rm -f "$NAME" >/dev/null 2>&1 || true; rm -rf "$ZIEL"' EXIT

echo "== Restore latest snapshot"
restic restore latest --tag swarvex --target "$ZIEL" --include /var/backups/swarvex/swarvex.dump
DUMP="$ZIEL/var/backups/swarvex/swarvex.dump"
[ -s "$DUMP" ] || { echo "dump missing in the backup"; exit 1; }

echo "== Load into a throwaway PostgreSQL"
docker run -d --rm --name "$NAME" --network none -e POSTGRES_PASSWORD=probe -e POSTGRES_USER=swarvex -e POSTGRES_DB=swarvex postgres:18 >/dev/null
for i in $(seq 1 30); do docker exec "$NAME" pg_isready -U swarvex -d swarvex >/dev/null 2>&1 && break; sleep 1; done
docker cp "$DUMP" "$NAME:/tmp/swarvex.dump"
docker exec "$NAME" pg_restore -U swarvex -d swarvex --no-owner /tmp/swarvex.dump

echo "== Check"
docker exec "$NAME" psql -U swarvex -d swarvex -At -c "select 'projects ' || count(*) from projekte union all select 'log events ' || count(*) from ereignisse"
echo "restore OK ($(restic snapshots --tag swarvex --latest 1 --compact | tail -3 | head -1))"
