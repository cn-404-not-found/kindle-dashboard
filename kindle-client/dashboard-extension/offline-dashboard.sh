#!/bin/sh
# Independent offline client: images update in the background, clock locally.
DIR=${KINDLE_DASHBOARD_DIR:-/mnt/us/extensions/kindle-dashboard}
STATE=${KINDLE_OFFLINE_STATE:-/tmp/kindle-dashboard-offline}
FBINK=${KINDLE_FBINK:-/mnt/us/libkh/bin/fbink}
. "$DIR/offline-network.sh" || exit 1
mkdir -p "$STATE" || exit 1
[ -x "$FBINK" ] || { echo 'FBInk not found'; exit 1; }
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
while :; do
  if [ -s "$STATE/ready.png" ]; then
    mv "$STATE/ready.png" "$STATE/current.png"
    eips -c
    eips -g "$STATE/current.png" || echo 'Failed to display background'
  fi
  # Fixed-cell font draws a white background over its previous text on each tick.
  "$FBINK" -q -S 9 -X 45 -Y 110 "$(date +%H:%M)" || echo 'Failed to draw clock'
  now=$(date +%s)
  delay=$((60 - now % 60))
  sleep "$delay" & wait $!
done
