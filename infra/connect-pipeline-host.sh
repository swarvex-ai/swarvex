#!/bin/sh
# Connects a prepared pipeline server to the platform and moves the jobs check and preview there.
# Run from your workstation, after infra/setup-pipeline-host.sh and infra/deploy-pipeline-host.sh:
#
#   sh infra/connect-pipeline-host.sh <platform-ssh-target> <pipeline-ssh-target> <pipeline-server-address>
#
# Safe to run again. Secrets travel from server to server through this pipe and are never printed.
# Copy built previews over first (they are only rebuilt after the next merge), for example:
#   ssh <platform> 'tar -C /var/lib/swarvex-pipeline/vorschau -cf - .' | ssh <pipeline> 'tar -C /var/lib/swarvex-pipeline/vorschau -xf -'
# Undo: delete the lines SWARVEX_JOBS, SWARVEX_PREVIEW_FROM and SWARVEX_PREVIEW_KEY from /opt/swarvex-data/pipeline.env
# on the platform, and on the pipeline server: systemctl disable --now swarvex-check.timer swarvex-preview.timer
set -eu
P="${1:?usage: infra/connect-pipeline-host.sh <platform> <pipeline-server> <pipeline-server-address>}"
H="${2:?usage: infra/connect-pipeline-host.sh <platform> <pipeline-server> <pipeline-server-address>}"
ADR="${3:?usage: infra/connect-pipeline-host.sh <platform> <pipeline-server> <pipeline-server-address>}"
STACK=/opt/swarvex/infra/stack
DATA=/opt/swarvex-data

echo "== 1. Platform: a key for the pipeline server, a key to fetch previews"
ssh "$P" sh -s <<EOF
set -eu
umask 077
cd $STACK
if ! grep -q '^PIPELINE_SECRET_CHECK=.' .env; then
  sed -i '/^PIPELINE_SECRET_CHECK=/d' .env
  echo "PIPELINE_SECRET_CHECK=\$(head -c 32 /dev/urandom | base64 | tr -d '/+=')" >> .env
  docker compose up -d api >/dev/null 2>&1
  echo "   check key created, API restarted"
fi
cd $DATA
[ -f pruefserver_vorschau ] || ssh-keygen -q -t ed25519 -N '' -C 'swarvex previews (read-only)' -f pruefserver_vorschau
ssh-keyscan -t ed25519 $ADR 2>/dev/null > pruefserver_vorschau.known_hosts
EOF
DORT=$(ssh "$P" "ssh-keygen -lf $DATA/pruefserver_vorschau.known_hosts | awk '{print \$2}'")
HIER=$(ssh-keygen -F "$ADR" -l | awk '$2 == "ED25519" {print $3}' | head -1)
if [ -z "$DORT" ] || [ "$DORT" != "$HIER" ]; then
  echo "   STOP: the pipeline server's host key differs between the platform ($DORT) and here ($HIER)"
  exit 1
fi
echo "   pipeline server host key matches: $HIER"

echo "== 2. Pipeline server: platform address and its key"
BASE=$(ssh "$P" "grep '^BASE_URL=' $STACK/.env | cut -d= -f2-")
ssh "$P" "grep '^PIPELINE_SECRET_CHECK=' $STACK/.env | cut -d= -f2-" | ssh "$H" "set -eu; umask 077; mkdir -p $DATA; read -r k; f=$DATA/pipeline.env; touch \$f; sed -i '/^SWARVEX_PLATFORM=/d;/^SWARVEX_PIPELINE_SECRET=/d' \$f; printf 'SWARVEX_PLATFORM=%s\nSWARVEX_PIPELINE_SECRET=%s\n' '$BASE' \"\$k\" >> \$f; chmod 600 \$f"
echo "   platform: $BASE"

echo "== 3. Platform: from now on only merging here, previews fetched from the pipeline server"
ssh "$P" "set -eu; f=$DATA/pipeline.env; sed -i '/^SWARVEX_JOBS=/d;/^SWARVEX_PREVIEW_FROM=/d;/^SWARVEX_PREVIEW_KEY=/d' \$f; printf 'SWARVEX_JOBS=merge\nSWARVEX_PREVIEW_FROM=vorschau@%s:\nSWARVEX_PREVIEW_KEY=%s/pruefserver_vorschau\n' '$ADR' '$DATA' >> \$f"

echo "== 3b. Previews the platform built since the first copy follow (it builds none from now on)"
V=/var/lib/swarvex-pipeline/vorschau
T=$(ssh "$H" "find $V -type f -printf '%T@\\n' 2>/dev/null | sort -n | tail -1 | cut -d. -f1")
ssh "$P" "cd $V && find . -type f -newermt '@${T:-0}' -print0 | tar --null -T - -cf -" | ssh "$H" "mkdir -p $V && tar -C $V -xf - && find $V -type d -exec chmod 755 {} +"
echo "   newer than $(date -d "@${T:-0}" '+%d.%m. %H:%M'): copied"

echo "== 4. Pipeline server: check and preview on, read-only access for the platform"
KEY=$(ssh "$P" "cat $DATA/pruefserver_vorschau.pub")
ssh "$H" "PREVIEW_SYNC_KEY='$KEY' sh /opt/swarvex/infra/setup-pipeline-host.sh" | tail -5

echo "== 5. Platform: fetch previews once"
ssh "$P" "systemctl start swarvex-preview.service; journalctl -u swarvex-preview.service -n 1 --no-pager -o cat"
