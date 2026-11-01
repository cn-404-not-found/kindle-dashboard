#!/bin/sh
# Wi-Fi helpers for the offline-clock client. The caller restores the original
# radio setting on exit and logs failures rather than hiding them.
network_wifi_set() {
  lipc-set-prop com.lab126.cmd wirelessEnable "$1"
}
network_wifi_state() {
  value=$(lipc-get-prop com.lab126.cmd wirelessEnable 2>/dev/null) || return 1
  case "$value" in 0|1) printf '%s\n' "$value" ;; *) return 1 ;; esac
}
network_wait_connected() {
  tries=0
  while [ "$tries" -lt "${KINDLE_WIFI_WAIT_SECONDS:-60}" ]; do
    status=$(wpa_cli -i wlan0 status 2>/dev/null)
    if printf '%s\n' "$status" | grep -q '^wpa_state=COMPLETED$' &&
       ifconfig wlan0 2>/dev/null | grep -q 'inet addr:'; then
      return 0
    fi
    sleep 1
    tries=$((tries + 1))
  done
  return 1
}

# Convert UTC offset to POSIX TZ syntax (the sign is intentionally reversed).
clock_tz() {
  offset=8
  [ -r "$1" ] && offset=$(cat "$1")
  case "$offset" in ''|*[!0-9-]*|--*|*--*|*-?*-) offset=8 ;; esac
  if ! [ "$offset" -ge -12 ] 2>/dev/null || ! [ "$offset" -le 14 ] 2>/dev/null; then offset=8; fi
  if [ "$offset" -ge 0 ]; then printf 'UTC-%s\n' "$offset"; else printf 'UTC+%s\n' "${offset#-}"; fi
}
# Clear the top area where Kindle's own status bar can redraw over the image.
clear_status_bar() {
  fbink=${KINDLE_FBINK:-/mnt/us/libkh/bin/fbink}
  if [ -x "$fbink" ]; then "$fbink" -q -k top=0,left=0,width=1072,height=48; else return 1; fi
}
