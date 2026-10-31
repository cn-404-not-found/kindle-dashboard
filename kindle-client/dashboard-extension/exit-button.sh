#!/bin/sh
# Read a separate copy of the Kindle power-key events without grabbing the device.
DEVICE=${KINDLE_POWER_DEVICE:-/dev/input/event0}
STATE=${KINDLE_DASHBOARD_STATE:-/tmp/kindle-dashboard-power}
SUPERVISOR=$1
case "$SUPERVISOR" in ''|*[!0-9]*) exit 1 ;; esac
[ -r "$DEVICE" ] || { echo "Power key not readable: $DEVICE"; exit 1; }
exec 3< "$DEVICE" || exit 1
reader=
cleanup() { [ -n "$reader" ] && kill "$reader" 2>/dev/null; rm -f "$STATE/key-event"; exit; }
trap cleanup TERM INT HUP
count=0
first=0
while :; do
  # Keep the event device open across reads so quick presses are not lost.
  dd bs=16 count=1 of="$STATE/key-event" <&3 2>/dev/null & reader=$!
  wait "$reader" || cleanup
  reader=
  set -- $(od -An -tu2 "$STATE/key-event")
  [ "$#" -ge 8 ] || cleanup
  # 32-bit input_event: EV_KEY=1, KEY_POWER=116, key down=1.
  if [ "$5" = 1 ] && [ "$6" = 116 ] && [ "$7" = 1 ] && [ "$8" = 0 ]; then
    now=$(date +%s)
    if [ "$count" -eq 0 ] || [ $((now - first)) -gt 5 ] || [ "$now" -lt "$first" ]; then
      first=$now
      count=0
    fi
    count=$((count + 1))
    if [ "$count" -ge 4 ]; then
      kill "$SUPERVISOR" 2>/dev/null
      cleanup
    fi
  fi
done
