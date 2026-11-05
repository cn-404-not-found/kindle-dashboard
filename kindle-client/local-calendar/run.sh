#!/bin/sh
DIR=/mnt/us/extensions/kindle-local-calendar
STATE=/tmp/kindle-local-calendar
mkdir -p "$STATE" || exit 1
cd "$DIR" || exit 1
# USB storage may be mounted noexec, so run the executable from /tmp.
cp "$DIR/calendar" "$STATE/calendar" && chmod 755 "$STATE/calendar" || exit 1
wm_pid=$(ps | awk '$NF == "awesome" {print $1; exit}')
guard=
case "$wm_pid" in
  ''|*[!0-9]*) wm_pid= ;;
  *) /bin/sh /mnt/us/extensions/kindle-dashboard/offline-ui-guard.sh "$$" "$wm_pid" & guard=$!
     kill -STOP "$wm_pid" 2>/dev/null || { kill "$guard" 2>/dev/null; guard=; wm_pid=; } ;;
esac
cleanup() {
  trap '' TERM INT HUP
  [ -n "$sleeper" ] && kill "$sleeper" 2>/dev/null
  [ -n "$wm_pid" ] && kill -CONT "$wm_pid" 2>/dev/null
  [ -n "$guard" ] && kill "$guard" 2>/dev/null
  exit 0
}
trap cleanup TERM INT HUP
while :; do
  # Use UTC+8 for both the rendered date and the midnight schedule.
  if "$STATE/calendar" --local-calendar "$STATE/screen.png"; then
    eips -c && eips -g "$STATE/screen.png" || echo 'Cannot draw calendar.' >&2
  else
    echo 'Calendar rendering failed; retrying in five minutes.' >&2
    sleep 300 & sleeper=$!; wait "$sleeper"; sleeper=; continue
  fi
  now=$(date +%s)
  delay=$((86400 - (now + 28800) % 86400))
  sleep "$delay" & sleeper=$!
  wait "$sleeper"
  sleeper=
done
