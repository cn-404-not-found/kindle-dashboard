#!/bin/sh
# Source from the landscape client; the detached guard resumes awesome if the
# client is killed before its cleanup trap runs.
landscape_ui_start() {
  [ -n "${landscape_wm_pid:-}" ] && return 0
  candidate=$(ps | awk '$NF == "awesome" {print $1; exit}')
  case "$candidate" in ''|*[!0-9]*) echo 'Window manager not found.' >&2; return 1 ;; esac
  mkdir -p "$TMP_DIR" || return 1
  cp "$KINDLE_DASHBOARD_DIR/offline-ui-guard.sh" "$TMP_DIR/landscape-ui-guard.sh" || return 1
  /bin/sh "$TMP_DIR/landscape-ui-guard.sh" "$$" "$candidate" & landscape_guard=$!
  if ! kill -STOP "$candidate" 2>/dev/null; then
    kill "$landscape_guard" 2>/dev/null
    wait "$landscape_guard" 2>/dev/null
    landscape_guard=
    echo 'Could not pause window manager.' >&2
    return 1
  fi
  landscape_wm_pid=$candidate
}

landscape_ui_cleanup() {
  trap '' TERM INT HUP
  [ -n "${landscape_sleep_pid:-}" ] && kill "$landscape_sleep_pid" 2>/dev/null
  if [ -n "${landscape_wm_pid:-}" ] && ! kill -CONT "$landscape_wm_pid" 2>/dev/null; then
    echo 'Could not resume window manager; recovery guard remains active.' >&2
    exit 1
  fi
  [ -n "${landscape_guard:-}" ] && kill "$landscape_guard" 2>/dev/null
  [ -n "${landscape_guard:-}" ] && wait "$landscape_guard" 2>/dev/null
  exit 0
}

landscape_sleep() {
  sleep "$1" & landscape_sleep_pid=$!
  wait "$landscape_sleep_pid"
  [ -n "$landscape_sleep_pid" ] && kill "$landscape_sleep_pid" 2>/dev/null
  [ -n "$landscape_sleep_pid" ] && wait "$landscape_sleep_pid" 2>/dev/null
  landscape_sleep_pid=
}
