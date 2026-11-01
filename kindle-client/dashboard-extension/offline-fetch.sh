#!/bin/sh
DIR=${KINDLE_DASHBOARD_DIR:-/mnt/us/extensions/kindle-dashboard}
STATE=${KINDLE_OFFLINE_STATE:-/tmp/kindle-dashboard-offline}
. "$DIR/offline-network.sh" || exit 1
mkdir -p "$STATE" || exit 1
last=0
last_date=
next_try=0
while :; do
  now=$(date +%s)
  today=$(date +%Y%m%d)
  if [ "$now" -ge "$next_try" ] && { [ "$last" -eq 0 ] || [ $((now-last)) -ge 7200 ] || [ "$today" != "$last_date" ] || [ "$now" -lt "$last" ]; }; then
    next_try=$((now + 300))
    url=$(/bin/sh "$DIR/server-settings.sh" --get)
    if [ -n "$url" ] && network_wifi_set 1 && network_wait_connected; then
      if curl -fsS --connect-timeout 10 --max-time 90 -D "$STATE/headers" -o "$STATE/download" \
          -H "png-width: ${KINDLE_SCREEN_WIDTH}" -H "png-height: ${KINDLE_SCREEN_HEIGHT}" \
          "${url}/screen.png?clock=local" && [ -s "$STATE/download" ] &&
          grep -iq '^X-Dashboard-Clock: local' "$STATE/headers"; then
        mv "$STATE/download" "$STATE/ready.png" && {
          last=$(date +%s)
          last_date=$(date +%Y%m%d)
          echo "$(date): background updated" >&2
        }
      else
        echo "$(date): background download failed or server lacks offline support" >&2
      fi
    else
      echo "$(date): Wi-Fi unavailable or server address invalid" >&2
    fi
    network_wifi_set 0 || echo "$(date): could not disable Wi-Fi" >&2
  fi
  sleep 30 & wait $!
done
