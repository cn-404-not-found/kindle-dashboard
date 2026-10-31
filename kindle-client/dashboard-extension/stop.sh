#!/bin/sh
PIDFILE=/tmp/kindle-dashboard.pid
if [ -f "$PIDFILE" ]; then
  PID=$(cat "$PIDFILE")
  case "$PID" in *[!0-9]*|"") ;; *) kill "$PID" 2>/dev/null ;; esac
  rm -f "$PIDFILE"
fi
