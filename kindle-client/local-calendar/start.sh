#!/bin/sh
# Reuse the existing power supervisor and exit gesture, without networking.
DIR=/mnt/us/extensions/kindle-dashboard
PIDFILE=/tmp/kindle-dashboard.pid
if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  echo 'Stop the running dashboard before starting the local calendar.' >> /tmp/kindle-local-calendar.log
  exit 1
fi
export KINDLE_DASHBOARD_PROGRAM=../kindle-local-calendar/run.sh
/bin/sh "$DIR/power.sh" >> /tmp/kindle-local-calendar.log 2>&1 </dev/null &
echo $! > "$PIDFILE"
