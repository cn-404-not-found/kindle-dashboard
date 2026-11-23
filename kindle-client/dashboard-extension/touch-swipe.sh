#!/bin/sh
# Turn horizontal swipes on the touchscreen into dashboard page requests.
# Usage: touch-swipe.sh [device] [state-dir] supervisor-pid
# Like touch-light.sh, this only reads the event device and never grabs it.
DEVICE=${1:-${KINDLE_TOUCH_DEVICE:-/dev/input/event1}}
STATE=${2:-${KINDLE_DASHBOARD_STATE:-/tmp/kindle-dashboard-power}}
SUPERVISOR=$3
AXIS=${KINDLE_SWIPE_AXIS:-y}
THRESHOLD=${KINDLE_SWIPE_THRESHOLD:-180}
DEBOUNCE=${KINDLE_SWIPE_DEBOUNCE:-2}
case "$SUPERVISOR" in ''|*[!0-9]*) exit 1 ;; esac
case "$THRESHOLD" in ''|*[!0-9]*) exit 1 ;; esac
case "$DEBOUNCE" in ''|*[!0-9]*) exit 1 ;; esac
case "$AXIS" in x|X) AXIS=x ;; y|Y) AXIS=y ;; *) exit 1 ;; esac
[ -r "$DEVICE" ] || { echo "Touch device not readable: $DEVICE" >&2; exit 1; }
mkdir -p "$STATE" || exit 1
exec 3< "$DEVICE" || exit 1

reader=
cleanup() {
  [ -n "$reader" ] && kill "$reader" 2>/dev/null
  rm -f "$STATE/swipe-event"
  exit 0
}
trap cleanup TERM INT HUP

start_x=-1
start_y=-1
cur_x=-1
cur_y=-1
touching=0
last_emit=0

# The landscape framebuffer is rotated 90 degrees, so a left/right finger
# movement arrives as a Y-axis change. KINDLE_SWIPE_AXIS=x covers unrotated
# layouts without touching the rest of the script.
begin() {
  touching=1
  start_x=-1
  start_y=-1
}

emit() {
  now=$(date +%s)
  [ $((now - last_emit)) -lt "$DEBOUNCE" ] && return
  printf '%s\n' "$1" > "$STATE/page-request.tmp" &&
    mv "$STATE/page-request.tmp" "$STATE/page-request" || return
  kill -USR1 "$SUPERVISOR" 2>/dev/null
  last_emit=$now
}

finish() {
  [ "$touching" -eq 1 ] || return
  touching=0
  [ "$start_x" -ge 0 ] && [ "$start_y" -ge 0 ] || return
  [ "$cur_x" -ge 0 ] && [ "$cur_y" -ge 0 ] || return
  dx=$((cur_x - start_x))
  dy=$((cur_y - start_y))
  adx=${dx#-}
  ady=${dy#-}
  if [ "$AXIS" = y ]; then
    [ "$ady" -ge "$THRESHOLD" ] && [ "$ady" -gt "$adx" ] || return
    if [ "$dy" -lt 0 ]; then emit next; else emit prev; fi
  else
    [ "$adx" -ge "$THRESHOLD" ] && [ "$adx" -gt "$ady" ] || return
    if [ "$dx" -lt 0 ]; then emit next; else emit prev; fi
  fi
}

while :; do
  dd bs=16 count=1 of="$STATE/swipe-event" <&3 2>/dev/null & reader=$!
  wait "$reader" || cleanup
  reader=
  set -- $(od -An -tu2 "$STATE/swipe-event")
  [ "$#" -ge 8 ] || cleanup
  type=$5
  code=$6
  value=$7
  if [ "$type" = 3 ]; then
    case "$code" in
      53|0)
        [ "$touching" -eq 1 ] || begin
        cur_x=$value
        [ "$start_x" -lt 0 ] && start_x=$value
        ;;
      54|1)
        [ "$touching" -eq 1 ] || begin
        cur_y=$value
        [ "$start_y" -lt 0 ] && start_y=$value
        ;;
      57)
        if [ "$value" = 65535 ]; then finish; else begin; fi
        ;;
    esac
  elif [ "$type" = 1 ] && [ "$code" = 330 ]; then
    if [ "$value" = 1 ]; then begin; else finish; fi
  fi
done
