#!/bin/sh
# Wi-Fi supervision for the online landscape client (sourced by
# landscape-partial.sh). The Kindle powers the radio down when it is idle or
# asleep, so the client must detect that and bring the link back instead of
# only retrying the HTTP request. POSIX sh only: the device uses ash.
#
# All checks are read-only when the link is healthy; writes happen only during
# recovery, so the normal one-minute refresh does not disturb Wi-Fi settings.

# Print the LIPC radio switch (0/1). Fails when the property is unreadable.
network_wifi_state() {
  value=$(lipc-get-prop com.lab126.cmd wirelessEnable 2>/dev/null) || return 1
  case "$value" in 0|1) printf '%s\n' "$value" ;; *) return 1 ;; esac
}

# Set the LIPC radio switch. Callers treat a failure as "will retry".
network_wifi_set() {
  lipc-set-prop com.lab126.cmd wirelessEnable "$1"
}

# True only when wpa_supplicant finished association and IPv4 is configured;
# a COMPLETED state without an address would still fail every curl.
network_associated() {
  status=$(wpa_cli -i wlan0 status 2>/dev/null) || return 1
  printf '%s\n' "$status" | grep -q '^wpa_state=COMPLETED$' || return 1
  ifconfig wlan0 2>/dev/null | grep -q 'inet addr:' || return 1
}

# Poll association for up to $1 seconds (default 60) before giving up.
network_wait_connected() {
  limit=${1:-${KINDLE_WIFI_WAIT_SECONDS:-60}}
  tries=0
  while [ "$tries" -lt "$limit" ]; do
    network_associated && return 0
    sleep 1
    tries=$((tries + 1))
  done
  return 1
}

# Cheap health check run before every fetch; performs no writes.
network_online() {
  [ "$(network_wifi_state)" = 1 ] || return 1
  network_associated
}

# Re-assert the screensaver block. If powerd drops it the device sleeps and
# powers the radio down, which looks exactly like a Wi-Fi outage from here.
network_keep_awake() {
  lipc-set-prop com.lab126.powerd preventScreenSaver 1 >/dev/null 2>&1
}

# Restore the link with the smallest step that works:
#   1. turn the radio back on if the device disabled it,
#   2. ask wpa_supplicant to reassociate (handles deauth / DHCP loss),
#   3. toggle the radio off and on (handles a wedged supplicant).
# Each attempted step is logged to stderr; returns 0 once the link is back.
network_ensure_online() {
  network_online && return 0
  echo 'Wi-Fi offline; attempting recovery.' >&2
  if [ "$(network_wifi_state 2>/dev/null)" != 1 ]; then
    echo 'Wi-Fi radio is off; enabling.' >&2
    network_wifi_set 1 >/dev/null 2>&1
  fi
  wpa_cli -i wlan0 reassociate >/dev/null 2>&1
  network_wait_connected "${KINDLE_WIFI_RECOVER_SECONDS:-20}" && return 0
  echo 'Reassociate failed; toggling the radio.' >&2
  network_wifi_set 0 >/dev/null 2>&1
  sleep 2
  network_wifi_set 1 >/dev/null 2>&1
  network_wait_connected "${KINDLE_WIFI_RECOVER_SECONDS:-20}" && return 0
  echo 'Wi-Fi recovery failed; retrying next cycle.' >&2
  return 1
}
