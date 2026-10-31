#!/bin/sh
# Dashboard power supervisor. Do not grab the touchscreen: KUAL remains usable.
DIR=${KINDLE_DASHBOARD_DIR:-/mnt/us/extensions/kindle-dashboard}
STATE=${KINDLE_DASHBOARD_STATE:-/tmp/kindle-dashboard-power}
LIPC=${KINDLE_LIPC_SET:-lipc-set-prop}
GET=${KINDLE_LIPC_GET:-lipc-get-prop}
TOUCH=${KINDLE_TOUCH_DEVICE:-/dev/input/event1}
LOG=${KINDLE_DASHBOARD_LOG:-/tmp/kindle-dashboard.log}
mkdir -p "$STATE" || exit 1
old_light=$($GET com.lab126.powerd flIntensity 2>/dev/null)
old_saver=$($GET com.lab126.powerd preventScreenSaver 2>/dev/null)
case "$old_light" in ''|*[!0-9]*) echo 'Cannot read frontlight; dashboard not started.' >> "$LOG"; exit 1 ;; esac
case "$old_saver" in 0|1) ;; *) echo 'Cannot read screensaver setting; dashboard not started.' >> "$LOG"; exit 1 ;; esac
if ! "$LIPC" com.lab126.powerd preventScreenSaver 1; then echo 'Cannot prevent screensaver.' >> "$LOG"; exit 1; fi
if ! "$LIPC" com.lab126.powerd flIntensity 0; then
  "$LIPC" com.lab126.powerd preventScreenSaver "$old_saver"
  echo 'Cannot turn down frontlight.' >> "$LOG"; exit 1
fi
cleanup() {
  trap '' TERM INT HUP
  [ -n "$button" ] && kill "$button" 2>/dev/null
  [ -n "$watcher" ] && kill "$watcher" 2>/dev/null
  [ -n "$dashboard" ] && kill "$dashboard" 2>/dev/null
  [ -n "$button" ] && wait "$button" 2>/dev/null
  [ -n "$watcher" ] && wait "$watcher" 2>/dev/null
  [ -n "$dashboard" ] && wait "$dashboard" 2>/dev/null
  "$LIPC" com.lab126.powerd flIntensity "$old_light" || echo 'Failed to restore frontlight.' >> "$LOG"
  "$LIPC" com.lab126.powerd preventScreenSaver "$old_saver" || echo 'Failed to restore screensaver.' >> "$LOG"
  rm -f "$STATE/touch" "$STATE/lit"
  "$LIPC" com.lab126.appmgrd start app://com.lab126.booklet.home || echo 'Failed to open Kindle home.' >> "$LOG"
  exit 0
}
trap cleanup TERM INT HUP
/bin/sh "$DIR/touch-light.sh" "$TOUCH" "$STATE" >> "$LOG" 2>&1 & watcher=$!
/bin/sh "$DIR/exit-button.sh" "$$" >> "$LOG" 2>&1 & button=$!
while :; do
  /bin/sh "$DIR/dashboard.sh" >> "$LOG" 2>&1 & dashboard=$!
  wait "$dashboard"
  dashboard=
  echo 'Dashboard exited; restarting in 5 seconds.' >> "$LOG"
  sleep 5 & wait $!
done
