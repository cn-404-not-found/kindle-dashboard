#!/bin/sh
DIR=${KINDLE_DASHBOARD_DIR:-/mnt/us/extensions/kindle-dashboard}
PIDFILE=/tmp/kindle-dashboard.pid
if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then exit 0; fi
export KINDLE_DASHBOARD_LAYOUT=landscape-v1
export KINDLE_DASHBOARD_PROGRAM=dashboard-landscape.sh
/bin/sh "$DIR/power.sh" >> /tmp/kindle-dashboard.log 2>&1 </dev/null &
echo $! > "$PIDFILE"
