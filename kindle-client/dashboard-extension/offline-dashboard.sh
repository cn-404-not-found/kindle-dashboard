#!/bin/sh
# Independent offline client: images update in the background, clock locally.
DIR=${KINDLE_DASHBOARD_DIR:-/mnt/us/extensions/kindle-dashboard}
STATE=${KINDLE_OFFLINE_STATE:-/tmp/kindle-dashboard-offline}
. "$DIR/offline-network.sh" || exit 1
mkdir -p "$STATE" || exit 1
initial_wifi=$(network_wifi_state) || { echo 'Cannot read Wi-Fi state'; exit 1; }
KINDLE_SCREEN_WIDTH=$(eips -i 2>/dev/null | sed -n 's/.*xres:[ ]*\([0-9]*\).*/\1/p' | head -1)
KINDLE_SCREEN_HEIGHT=$(eips -i 2>/dev/null | sed -n 's/.*yres:[ ]*\([0-9]*\).*/\1/p' | head -1)
export KINDLE_SCREEN_WIDTH KINDLE_SCREEN_HEIGHT
case "$KINDLE_SCREEN_WIDTH:$KINDLE_SCREEN_HEIGHT" in 1072:1448) ;; *) echo "Unsupported framebuffer: $KINDLE_SCREEN_WIDTH x $KINDLE_SCREEN_HEIGHT"; exit 1 ;; esac
cleanup() {
  trap '' TERM INT HUP
  [ -n "$worker" ] && kill "$worker" 2>/dev/null
  [ -n "$worker" ] && wait "$worker" 2>/dev/null
  [ -n "$wm_pid" ] && kill -CONT "$wm_pid" 2>/dev/null
  [ -n "$ui_guard" ] && kill "$ui_guard" 2>/dev/null
  [ -n "$ui_guard" ] && wait "$ui_guard" 2>/dev/null
  network_wifi_set "$initial_wifi" || echo 'Could not restore original Wi-Fi setting'
  exit 0
}
trap cleanup TERM INT HUP
# A finished download interrupts the minute wait so the new image appears promptly.
trap : USR1
KINDLE_OFFLINE_DISPLAY_PID=$$
export KINDLE_OFFLINE_DISPLAY_PID
# Pause only the window manager, not networking; a separate guard resumes it if
# this process is killed unexpectedly. Raw power-button input remains active.
wm_pid=$(ps | awk '$NF == "awesome" {print $1; exit}')
case "$wm_pid" in
  ''|*[!0-9]*) wm_pid= ;;
  *)
    if cp "$DIR/offline-ui-guard.sh" "$STATE/ui-guard.sh"; then
      /bin/sh "$STATE/ui-guard.sh" "$$" "$wm_pid" & ui_guard=$!
      if ! kill -STOP "$wm_pid" 2>/dev/null; then
        kill "$ui_guard" 2>/dev/null; wait "$ui_guard" 2>/dev/null
        ui_guard=; wm_pid=
        echo 'Unable to pause window manager'
      fi
    else
      wm_pid=
      echo 'Unable to start window manager recovery guard'
    fi ;;
esac
/bin/sh "$DIR/offline-fetch.sh" & worker=$!
active=
previous=
bundle_is_valid() {
  [ -s "$1/background.png" ] || return 1
  for glyph in 0 1 2 3 4 5 6 7 8 9 colon; do
    [ -s "$1/$glyph.png" ] || return 1
  done
}
display_bundle() {
  candidate=$1
  bundle_is_valid "$candidate" || return 1
  old_active=$active
  eips -c
  if ! eips -g "$candidate/background.png"; then
    echo 'Failed to display background'
    return 1
  fi
  active=$candidate
  if [ -s "$active/timezone" ]; then
    cp "$active/timezone" "$STATE/clock-offset"
  fi
  printf '%s\n' "${candidate##*/}" > "$STATE/active.tmp" && mv "$STATE/active.tmp" "$STATE/active"
  case "$old_active" in "$STATE"/bundle-[0-9]*)
    [ "$old_active" = "$active" ] || rm -rf "$old_active" ;;
  esac
  previous=
}
# Restore the last complete bundle before waiting for a fresh network request.
if [ -s "$STATE/active" ]; then
  saved=$(cat "$STATE/active")
  case "$saved" in bundle-*[!0-9]*|*[!a-z0-9-]*|'') ;; bundle-*) display_bundle "$STATE/$saved" ;; esac
fi
if [ -z "$active" ]; then
  for candidate in "$STATE"/bundle-*; do
    [ -d "$candidate" ] && bundle_is_valid "$candidate" && display_bundle "$candidate"
  done
fi
if [ -z "$active" ]; then
  eips -c
  eips 0 0 'Connecting to dashboard server...'
fi
while :; do
  if [ -s "$STATE/ready" ]; then
    bundle=$(cat "$STATE/ready")
    rm -f "$STATE/ready"
    case "$bundle" in bundle-*[!0-9]*|*[!a-z0-9-]*|'') echo 'Invalid bundle name' ;;
      bundle-*) display_bundle "$STATE/$bundle" || echo 'Incomplete dashboard bundle' ;;
    esac
  fi
  if [ -n "$active" ]; then
    clock_y=110
    if [ -f "$active/clock-y" ] && [ "$(cat "$active/clock-y")" = 69 ]; then clock_y=69; fi
    current=$(TZ="$(clock_tz "$STATE/clock-offset")" date +%H:%M)
    pos=1
    while [ "$pos" -le 5 ]; do
      digit=$(printf '%s' "$current" | cut -c "$pos")
      old=$(printf '%s' "$previous" | cut -c "$pos")
      if [ "$digit" != "$old" ]; then
        [ "$digit" = : ] && digit=colon
        x=$((45 + (pos - 1) * 64))
        eips -g "$active/$digit.png" -x "$x" -y "$clock_y" || echo "Failed to draw clock at $pos"
      fi
      pos=$((pos + 1))
    done
    previous=$current
    clear_status_bar || echo 'Could not clear status bar'
  fi
  now=$(date +%s)
  delay=$((60 - now % 60))
  sleep "$delay" & sleeper=$!
  wait "$sleeper"
  kill "$sleeper" 2>/dev/null
done
