#!/bin/sh
# Linux input_event on the 32-bit Kindle has 16 bytes. Read only this device,
# without EVIOCGRAB, so input is still available to the Kindle interface.
DEVICE=$1
STATE=$2
LIPC=${KINDLE_LIPC_SET:-lipc-set-prop}
LIGHT=${KINDLE_TOUCH_BRIGHTNESS:-12}
TIMEOUT=${KINDLE_TOUCH_TIMEOUT:-30}
case "$LIGHT:$TIMEOUT" in *[!0-9:]*|:*|*:) exit 1 ;; esac
[ "$LIGHT" -ge 1 ] && [ "$LIGHT" -le 24 ] && [ "$TIMEOUT" -ge 1 ] || exit 1
[ -r "$DEVICE" ] || { echo "Touch device not readable: $DEVICE"; exit 1; }
# The timer is separate from the blocking input reader.
(
  while :; do
    if [ -f "$STATE/touch" ]; then
      last=$(cat "$STATE/touch" 2>/dev/null)
      now=$(date +%s)
      case "$last" in ''|*[!0-9]*) last=0 ;; esac
      if [ $((now - last)) -ge "$TIMEOUT" ] && [ -f "$STATE/lit" ]; then
        "$LIPC" com.lab126.powerd flIntensity 0 && rm -f "$STATE/lit"
      fi
    fi
    sleep 1
done
) & timer=$!
cleanup() { [ -n "$reader" ] && kill "$reader" 2>/dev/null; kill "$timer" 2>/dev/null; wait "$timer" 2>/dev/null; rm -f "$STATE/event"; exit; }
trap cleanup TERM INT HUP
while :; do
  # Each event contains timeval (8 bytes), type, code and value.
  dd if="$DEVICE" of="$STATE/event" bs=16 count=1 2>/dev/null & reader=$!
  wait "$reader" || cleanup
  reader=
  set -- $(od -An -tu2 "$STATE/event")
  [ "$#" -ge 8 ] || { echo 'Touch input closed.'; cleanup; }
  # ABS events are emitted by the touchscreen for contact and movement.
  if [ "$5" = 3 ]; then
    date +%s > "$STATE/touch"
    if [ ! -f "$STATE/lit" ]; then
      "$LIPC" com.lab126.powerd flIntensity "$LIGHT" && : > "$STATE/lit"
    fi
  fi
done
