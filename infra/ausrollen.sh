#!/bin/sh
# Bringt den aktuellen Stand auf die Plattform-VM und baut API und Website neu.
# Aufruf vom Arbeitsrechner aus: infra/ausrollen.sh <ssh-ziel>
#
# Zwei Schritte, weil auf der VM auch Dateien liegen, die es nur dort gibt (.env, Schlüssel, Medien):
#  1. Das ganze Repo ohne --delete (nichts auf der VM geht verloren).
#  2. Reine Quellcode-Ordner mit --delete, damit verschobene oder gelöschte Dateien dort nicht liegen bleiben.
set -eu
ZIEL="${1:?Aufruf: infra/ausrollen.sh <ssh-ziel>}"
REPO="$(cd "$(dirname "$0")/.." && pwd)"
AUSNAHMEN="--exclude node_modules --exclude .git --exclude .venv* --exclude media/ --exclude ausgabe/ --exclude .svelte-kit --exclude build/ --exclude stimme/*.onnx* --exclude stimme/referenz_* --exclude entwuerfe/ --exclude .env.local --exclude .env.*.local"

# shellcheck disable=SC2086
rsync -az $AUSNAHMEN "$REPO/" "root@$ZIEL:/opt/swarvex/" 2>/dev/null || rsync -az $AUSNAHMEN "$REPO/" "$ZIEL:/opt/swarvex/"
for ordner in apps/web/src apps/api/src lead docs; do
  rsync -az --delete "$REPO/$ordner/" "$ZIEL:/opt/swarvex/$ordner/"
done
if [ -d "$REPO/apps/web/static/media" ]; then
  rsync -az "$REPO/apps/web/static/media/" "$ZIEL:/opt/swarvex-daten/downloads/media/"
fi
ssh "$ZIEL" 'cd /opt/swarvex/infra/stack && docker compose build api web && docker compose up -d api web caddy && docker compose restart caddy'
