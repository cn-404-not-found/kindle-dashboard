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
  network_wifi_set "$initial_wifi" || echo 'Could not restore original Wi-Fi setting'
  exit 0
}
trap cleanup TERM INT HUP
/bin/sh "$DIR/offline-fetch.sh" & worker=$!
active=
previous=
while :; do
  if [ -s "$STATE/ready" ]; then
    bundle=$(cat "$STATE/ready")
    rm -f "$STATE/ready"
    case "$bundle" in *[!a-z0-9-]*|'') echo 'Invalid bundle name' ;;
      bundle-*)
        if [ -s "$STATE/$bundle/background.png" ]; then
          old_active=$active
          active="$STATE/$bundle"
          eips -c
          eips -g "$active/background.png" || echo 'Failed to display background'
          case "$old_active" in "$STATE"/bundle-[0-9]*) rm -rf "$old_active" ;; esac
          previous=
        fi ;;
    esac
  fi
  if [ -n "$active" ]; then
    current=$(date +%H:%M)
    pos=1
    while [ "$pos" -le 5 ]; do
      digit=$(printf '%s' "$current" | cut -c "$pos")
      old=$(printf '%s' "$previous" | cut -c "$pos")
      if [ "$digit" != "$old" ]; then
        [ "$digit" = : ] && digit=colon
        x=$((45 + (pos - 1) * 64))
        eips -g "$active/$digit.png" -x "$x" -y 110 || echo "Failed to draw clock at $pos"
      fi
      pos=$((pos + 1))
    done
    previous=$current
  fi
  now=$(date +%s)
  delay=$((60 - now % 60))
  sleep "$delay" & wait $!
done
