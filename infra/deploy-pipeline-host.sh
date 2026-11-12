#!/bin/sh
# Brings the current test pipeline to a separate pipeline server (infra/setup-pipeline-host.sh) and rebuilds it.
# Run from your workstation: infra/deploy-pipeline-host.sh <ssh-target>
#
# Only what the pipeline needs: its code, the runner module it shares the environment package with, the lead
# templates the environment detection reads, and the systemd units. No platform secrets go there.
set -eu
set -f
TARGET="${1:?usage: infra/deploy-pipeline-host.sh <ssh-target>}"
REPO="$(cd "$(dirname "$0")/.." && pwd)"
ssh "$TARGET" 'mkdir -p /opt/swarvex/infra'
for dir in pipeline runner lead infra/systemd; do
  rsync -az --delete --exclude .git --exclude node_modules "$REPO/$dir/" "$TARGET:/opt/swarvex/$dir/"
done
rsync -az "$REPO/infra/setup-pipeline-host.sh" "$REPO/infra/pipeline-metrics.sh" "$TARGET:/opt/swarvex/infra/"
ssh "$TARGET" 'cd /opt/swarvex && docker run --rm -v "$PWD:/src" -w /src/pipeline -e CGO_ENABLED=0 -e GOCACHE=/tmp/gc docker.io/library/golang:1 go build -trimpath -ldflags="-s -w" -o swarvex-pipeline . && install -m 755 pipeline/swarvex-pipeline /usr/local/bin/ && rm -f pipeline/swarvex-pipeline && install -m 644 infra/systemd/swarvex-check.service infra/systemd/swarvex-check.timer infra/systemd/swarvex-preview.service infra/systemd/swarvex-preview.timer /etc/systemd/system/ && systemctl daemon-reload && echo "pipeline built"'
