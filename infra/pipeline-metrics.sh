#!/bin/sh
# Metrics of one test pipeline run for Prometheus, as a node exporter textfile (like infra/backup.sh).
# Called by the systemd units after every run: ExecStopPost=-/bin/sh /opt/swarvex/infra/pipeline-metrics.sh <task>
# systemd sets SERVICE_RESULT there; the unit's EnvironmentFile gives SWARVEX_JOBS, SWARVEX_RUNTIME and
# SWARVEX_PREVIEW_FROM. A task that runs on another server writes nothing here; a platform that fetches its previews
# from the pipeline server reports that as the task preview_fetch.
set -eu
TASK="${1:?usage: pipeline-metrics.sh check|merge|preview}"
D=/var/lib/swarvex-metrics
mkdir -p "$D"
case ",$(printf '%s' "${SWARVEX_JOBS:-$TASK}" | tr -d ' ')," in
  *",$TASK,"*) T="$TASK" ;;
  *) if [ "$TASK" = preview ] && [ -n "${SWARVEX_PREVIEW_FROM:-}" ]; then T=preview_fetch; else rm -f "$D/pipeline_$TASK.prom" "$D/pipeline_${TASK}_success.prom"; exit 0; fi ;;
esac
OK=0
[ "${SERVICE_RESULT:-}" = success ] && OK=1
JETZT=$(date +%s)
# Duration of the main process in microseconds (monotonic clock)
set -- $(systemctl show "swarvex-$TASK.service" -p ExecMainStartTimestampMonotonic -p ExecMainExitTimestampMonotonic --value 2>/dev/null) 0 0
DAUER=$(awk -v a="$1" -v b="$2" 'BEGIN { d = (b - a) / 1e6; if (d < 0) d = 0; printf "%.3f", d }')
umask 022
{
  echo "# HELP swarvex_pipeline_last_run_timestamp_seconds End of the last run of a test pipeline task"
  echo "# TYPE swarvex_pipeline_last_run_timestamp_seconds gauge"
  echo "swarvex_pipeline_last_run_timestamp_seconds{task=\"$T\"} $JETZT"
  echo "# HELP swarvex_pipeline_last_run_success 1 if the last run of the task succeeded, 0 if it failed"
  echo "# TYPE swarvex_pipeline_last_run_success gauge"
  echo "swarvex_pipeline_last_run_success{task=\"$T\"} $OK"
  echo "# HELP swarvex_pipeline_last_run_duration_seconds Duration of the last run of the task"
  echo "# TYPE swarvex_pipeline_last_run_duration_seconds gauge"
  echo "swarvex_pipeline_last_run_duration_seconds{task=\"$T\"} $DAUER"
  if [ "$T" = check ]; then
    echo "# HELP swarvex_pipeline_runtime_info Container runtime around the checks (runsc: gVisor)"
    echo "# TYPE swarvex_pipeline_runtime_info gauge"
    echo "swarvex_pipeline_runtime_info{runtime=\"${SWARVEX_RUNTIME:-runc}\"} 1"
  fi
} > "$D/pipeline_$T.prom.tmp"
mv "$D/pipeline_$T.prom.tmp" "$D/pipeline_$T.prom"
if [ "$OK" = 1 ]; then
  {
    echo "# HELP swarvex_pipeline_last_success_timestamp_seconds End of the last successful run of the task"
    echo "# TYPE swarvex_pipeline_last_success_timestamp_seconds gauge"
    echo "swarvex_pipeline_last_success_timestamp_seconds{task=\"$T\"} $JETZT"
  } > "$D/pipeline_${T}_success.prom.tmp"
  mv "$D/pipeline_${T}_success.prom.tmp" "$D/pipeline_${T}_success.prom"
fi
