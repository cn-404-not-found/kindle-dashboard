#!/bin/sh
PIDFILE=/tmp/kindle-dashboard.pid
if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then exit 0; fi
/bin/sh /mnt/us/extensions/kindle-dashboard/dashboard.sh >> /tmp/kindle-dashboard.log 2>&1 </dev/null &
echo $! > "$PIDFILE"
