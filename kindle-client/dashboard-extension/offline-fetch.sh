#!/bin/sh
DIR=${KINDLE_DASHBOARD_DIR:-/mnt/us/extensions/kindle-dashboard}
STATE=${KINDLE_OFFLINE_STATE:-/tmp/kindle-dashboard-offline}
. "$DIR/offline-network.sh" || exit 1
mkdir -p "$STATE" || exit 1
last_slot=
last_url=
next_try=0
while :; do
  now=$(date +%s)
  slot=$(clock_two_hour_slot "$now" "$STATE/clock-offset")
  url=$(/bin/sh "$DIR/server-settings.sh" --get)
  if [ "$now" -ge "$next_try" ] && { [ "$slot" != "$last_slot" ] || [ "$url" != "$last_url" ]; }; then
    next_try=$((now + 300))
    if [ -n "$url" ] && network_wifi_set 1 && network_wait_connected; then
      bundle="$STATE/bundle-$now"
      mkdir -p "$bundle"
      if curl -fsS --connect-timeout 10 --max-time 90 -D "$STATE/headers" -o "$bundle/background.png" \
          -H "png-width: ${KINDLE_SCREEN_WIDTH}" -H "png-height: ${KINDLE_SCREEN_HEIGHT}" \
          "${url}/screen.png?clock=local" && [ -s "$bundle/background.png" ] &&
          grep -iq '^X-Dashboard-Clock: local' "$STATE/headers" &&
          curl -fsS --connect-timeout 10 --max-time 90 -o "$bundle/tiles.tar" "${url}/clock-tiles.tar" &&
          [ "$(tar -tf "$bundle/tiles.tar" | LC_ALL=C sort)" = "$(printf '%s\n' 0.png 1.png 2.png 3.png 4.png 5.png 6.png 7.png 8.png 9.png colon.png | LC_ALL=C sort)" ] &&
          tar -xf "$bundle/tiles.tar" -C "$bundle" &&
          awk 'tolower($1)=="x-dashboard-timezone-offset:" {gsub("\r", "", $2); print $2; exit}' "$STATE/headers" > "$bundle/timezone" &&
          printf '%s\n' "bundle-$now" > "$STATE/ready.tmp" && mv "$STATE/ready.tmp" "$STATE/ready"; then
        last_slot=$(clock_two_hour_slot "$(date +%s)" "$bundle/timezone")
        last_url=$url
        next_try=0
        case "${KINDLE_OFFLINE_DISPLAY_PID:-}" in ''|*[!0-9]*) ;; *) kill -USR1 "$KINDLE_OFFLINE_DISPLAY_PID" 2>/dev/null ;; esac
        echo "$(date): background and clock font updated" >&2
      else
        echo "$(date): background download failed or server lacks offline support" >&2
        rm -rf "$bundle"
      fi
    else
      echo "$(date): Wi-Fi unavailable or server address invalid" >&2
    fi
    network_wifi_set 0 || echo "$(date): could not disable Wi-Fi" >&2
    sleep 2
    clear_status_bar || echo "$(date): could not clear status bar" >&2
  fi
  now=$(date +%s)
  sleep $((30 - now % 30)) & wait $!
done
