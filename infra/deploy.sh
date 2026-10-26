#!/bin/sh
# Brings the current state to the platform VM and rebuilds API and website.
# Run from your workstation: infra/deploy.sh <ssh-target>
#
# Two steps, because the VM also holds files that only exist there (.env, keys, media):
#  1. The whole repo without --delete (nothing on the VM gets lost).
#  2. Pure source folders with --delete, so moved or deleted files do not linger there.
set -eu
TARGET="${1:?usage: infra/deploy.sh <ssh-target>}"
REPO="$(cd "$(dirname "$0")/.." && pwd)"
EXCLUDES="--exclude node_modules --exclude .git --exclude .venv* --exclude media/ --exclude output/ --exclude ausgabe/ --exclude .svelte-kit --exclude build/ --exclude .env.local --exclude .env.*.local"

# shellcheck disable=SC2086
rsync -az $EXCLUDES "$REPO/" "root@$TARGET:/opt/swarvex/" 2>/dev/null || rsync -az $EXCLUDES "$REPO/" "$TARGET:/opt/swarvex/"
for dir in apps/web/src apps/api/src lead docs runner pipeline compat; do
  rsync -az --delete $EXCLUDES "$REPO/$dir/" "$TARGET:/opt/swarvex/$dir/"
done
if [ -d "$REPO/apps/web/static/media" ]; then
  rsync -az "$REPO/apps/web/static/media/" "$TARGET:/opt/swarvex-data/downloads/media/"
fi
ssh "$TARGET" 'cd /opt/swarvex/infra/stack && docker compose build api web && docker compose up -d api web caddy && docker compose restart caddy'
