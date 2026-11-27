#!/bin/sh
# Wi-Fi supervision for the online landscape client (sourced by
# landscape-partial.sh). The Kindle powers the radio down when it is idle or
# asleep, so the client must detect that and bring the link back instead of
# only retrying the HTTP request. POSIX sh only: the device uses ash.
#
# All checks are read-only when the link is healthy; writes happen only during
# recovery, so the normal one-minute refresh does not disturb Wi-Fi settings.

# Config locations for kindle-network static-IP helper; tests may override these.
NET_DIR=${KINDLE_NET_CONFIG_DIR:-/mnt/us/extensions/kindle-network}
NET_AUTO=${KINDLE_NET_AUTO:-$NET_DIR/auto.sh}

# Log timestamped recovery/failure diagnostics to stderr and optionally persist
# recent lines to KINDLE_WIFI_EVENT_LOG (capped at 200 lines) for post-mortem.
network_log() {
  ts=$(date '+%Y-%m-%d %H:%M:%S' 2>/dev/null)
  [ -n "$ts" ] || ts='unknown-time'
  line="[$ts] $*"
  printf '%s\n' "$line" >&2
  log_file=${KINDLE_WIFI_EVENT_LOG:-}
  [ -n "$log_file" ] || return 0
  log_dir=$(dirname "$log_file" 2>/dev/null)
  [ -n "$log_dir" ] && [ ! -d "$log_dir" ] && mkdir -p "$log_dir" 2>/dev/null
  printf '%s\n' "$line" >> "$log_file" 2>/dev/null || return 0
  lines=$(wc -l < "$log_file" 2>/dev/null)
  case "$lines" in
    ''|*[!0-9]*) ;;
    *)
      if [ "$lines" -gt 220 ]; then
        tail -n 200 "$log_file" > "${log_file}.tmp" 2>/dev/null && \
          mv "${log_file}.tmp" "$log_file" 2>/dev/null || \
          rm -f "${log_file}.tmp" 2>/dev/null
      fi
      ;;
  esac
}

# Print the LIPC radio switch (0/1). Fails when the property is unreadable.
network_wifi_state() {
  value=$(lipc-get-prop com.lab126.cmd wirelessEnable 2>/dev/null) || return 1
  case "$value" in 0|1) printf '%s\n' "$value" ;; *) return 1 ;; esac
}

# Set the LIPC radio switch. Callers treat a failure as "will retry".
network_wifi_set() {
  lipc-set-prop com.lab126.cmd wirelessEnable "$1"
}

# True when wpa_supplicant finished L2 association (even before IPv4 is ready).
network_wpa_completed() {
  status=$(wpa_cli -i wlan0 status 2>/dev/null) || return 1
  printf '%s\n' "$status" | grep -q '^wpa_state=COMPLETED$'
}

# True only when wpa_supplicant finished association and IPv4 is configured;
# a COMPLETED state without an address would still fail every curl.
network_associated() {
  network_wpa_completed || return 1
  ifconfig wlan0 2>/dev/null | grep -q 'inet addr:' || return 1
}

# True when kindle-network is configured for static IP assignment.
network_static_active() {
  [ -r "$NET_DIR/mode" ] || return 1
  mode=$(tr -d ' \t\r\n' < "$NET_DIR/mode" 2>/dev/null)
  [ "$mode" = static ]
}

# Re-apply static IP/gateway/DNS and ensure auto.sh's monitor loop is running
# (recovers immediately when USB unmount or reboot stopped auto.sh).
network_apply_static() {
  network_static_active || return 0
  [ -f "$NET_AUTO" ] || return 0
  /bin/sh "$NET_AUTO" --ensure </dev/null >/dev/null 2>&1
  /bin/sh "$NET_AUTO" --tick </dev/null >/dev/null 2>&1
}

# Poll association for up to $1 seconds (default 60) before giving up.
# In static mode, invoke auto.sh immediately once COMPLETED instead of waiting
# for the external cron/loop interval.
network_wait_connected() {
  limit=${1:-${KINDLE_WIFI_WAIT_SECONDS:-60}}
  tries=0
  while [ "$tries" -lt "$limit" ]; do
    network_associated && return 0
    if network_wpa_completed && network_static_active; then
      network_apply_static
      network_associated && return 0
    fi
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

# Enable wireless daemons and ask wpa_supplicant to reconnect/reassociate.
network_kick_radio() {
  network_wifi_set 1 >/dev/null 2>&1
  lipc-set-prop com.lab126.wifid enable 1 >/dev/null 2>&1
  lipc-set-prop com.lab126.cmd ensureConnection wifi >/dev/null 2>&1
  wpa_cli -i wlan0 enable_network all >/dev/null 2>&1
  wpa_cli -i wlan0 reconnect >/dev/null 2>&1
  wpa_cli -i wlan0 reassociate >/dev/null 2>&1
}

# Full radio toggle used when soft reconnect fails or repeated curl errors occur.
network_cycle_radio() {
  network_wifi_set 0 >/dev/null 2>&1
  lipc-set-prop com.lab126.wifid enable 0 >/dev/null 2>&1
  sleep 2
  network_kick_radio
}

# Restore the link with the smallest step that works:
#   1. if already COMPLETED in static mode without IPv4, invoke auto.sh first,
#   2. turn wireless/wifid on and ask wpa_supplicant to reconnect/reassociate,
#   3. toggle the radio off and on (handles a wedged supplicant/driver).
network_ensure_online() {
  network_online && return 0
  network_log 'Wi-Fi offline; attempting recovery.'
  if [ "$(network_wifi_state 2>/dev/null)" = 1 ] && network_wpa_completed && network_static_active; then
    network_log 'Associated without IPv4 in static mode; invoking auto.sh.'
    network_apply_static
    if network_online; then
      network_log 'Static IP restored without restarting radio.'
      return 0
    fi
  fi
  if [ "$(network_wifi_state 2>/dev/null)" != 1 ]; then
    network_log 'Wi-Fi radio is off; enabling.'
  fi
  network_kick_radio
  if network_wait_connected "${KINDLE_WIFI_RECOVER_SECONDS:-30}"; then
    network_static_active && network_apply_static
    network_log 'Wi-Fi recovered via soft reconnect.'
    return 0
  fi
  network_log 'Soft reconnect failed; toggling the radio.'
  network_cycle_radio
  if network_wait_connected "${KINDLE_WIFI_RECOVER_SECONDS:-30}"; then
    network_static_active && network_apply_static
    network_log 'Wi-Fi recovered after radio toggle.'
    return 0
  fi
  network_log 'Wi-Fi recovery failed; retrying next cycle.'
  return 1
}

# Force recovery even when local link state looks online (e.g. static IP stays
# bound on wlan0 while route, AP session, or driver TX queue is wedged).
# Escalates to a radio power-cycle at streak >= 6.
network_force_recover() {
  streak=${1:-3}
  network_log "Consecutive fetch failures ($streak); forcing Wi-Fi recovery."
  network_static_active && network_apply_static
  network_kick_radio
  if [ "$streak" -ge 6 ] || ! network_wait_connected "${KINDLE_WIFI_RECOVER_SECONDS:-30}"; then
    network_log "Escalating to radio toggle (streak=$streak)."
    network_cycle_radio
    if ! network_wait_connected "${KINDLE_WIFI_RECOVER_SECONDS:-30}"; then
      network_log 'Forced Wi-Fi recovery failed; retrying later.'
      return 1
    fi
  fi
  network_static_active && network_apply_static
  network_log 'Forced Wi-Fi recovery completed.'
  return 0
}
