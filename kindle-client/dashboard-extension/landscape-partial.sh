#!/bin/sh
# PW2/PW3 landscape online mode: full paint on startup/day change, partial paints otherwise.
DIR=${KINDLE_DASHBOARD_DIR:-/mnt/us/extensions/kindle-dashboard}
KINDLE_DASHBOARD_DIR=$DIR
TMP_DIR=${KINDLE_PARTIAL_STATE:-/tmp/kindle-landscape-partial}
mkdir -p "$TMP_DIR" || exit 1
. "$DIR/landscape-ui.sh" || exit 1
. "$DIR/network-health.sh" || exit 1
trap landscape_ui_cleanup TERM INT HUP

width=$(eips -i 2>/dev/null | sed -n 's/.*xres:[ ]*\([0-9]*\).*/\1/p' | head -1)
height=$(eips -i 2>/dev/null | sed -n 's/.*yres:[ ]*\([0-9]*\).*/\1/p' | head -1)
case "$width:$height" in
  1072:1448) clock_x=852; clock_y=55; weather_x=182; weather_y=55 ;;
  758:1024) clock_x=602; clock_y=38; weather_x=128; weather_y=38 ;;
  *) echo "Unsupported framebuffer: $width x $height" >&2; exit 1 ;;
esac

header() { awk -v key="$1" 'tolower($1)==tolower(key ":") {gsub("\r", "", $2); print $2; exit}' "$TMP_DIR/headers"; }
valid_metadata() {
  [ "$(header X-Dashboard-Layout)" = landscape-v1 ] &&
  [ "$(header X-Dashboard-Partial-Version)" = 1 ] || return 1
  current_day=$(header X-Dashboard-Date)
  current_hour=$(header X-Dashboard-Hour)
  case "$current_day" in ????-??-??) ;; *) return 1 ;; esac
  case "$current_hour" in ??????????) ;; *) return 1 ;; esac
  case "$current_day$current_hour" in *[!0-9-]*) return 1 ;; esac
}
fetch() {
  suffix=$1
  curl -fsS --connect-timeout 10 --max-time 45 -D "$TMP_DIR/headers" -o "$TMP_DIR/image.png" \
    -H "png-width: $width" -H "png-height: $height" -H 'X-Dashboard-Layout: landscape-v1' \
    "${url}/screen.png${suffix}" && [ -s "$TMP_DIR/image.png" ] && valid_metadata
}
show_full() {
  if ! fetch ''; then echo 'Full image unavailable or server lacks partial support.' >&2; return 1; fi
  if ! landscape_ui_start; then echo 'Could not suppress status bar.' >&2; return 1; fi
  eips -c && landscape_sleep 1 && eips -g "$TMP_DIR/image.png" || return 1
  displayed_day=$current_day
  displayed_hour=$current_hour
}
show_patch() {
  part=$1; expected_x=$2; expected_y=$3
  fetch "?part=$part" || return 1
  [ "$(header X-Dashboard-Part)" = "$part" ] && [ "$(header X-Dashboard-X)" = "$expected_x" ] && [ "$(header X-Dashboard-Y)" = "$expected_y" ] || return 1
  [ "$current_day" = "$displayed_day" ] || return 2
  eips -g "$TMP_DIR/image.png" -x "$expected_x" -y "$expected_y"
}

displayed_day=
displayed_hour=
while :; do
  url=$(/bin/sh "$DIR/server-settings.sh" --get) || url=
  if [ -n "$url" ]; then
    if [ "${KINDLE_WIFI_WATCHDOG:-1}" != 0 ]; then
      # Keep the device awake and bring the radio back before touching curl.
      network_keep_awake
      network_ensure_online
    fi
    if [ -z "$displayed_day" ]; then
      show_full || echo 'Full display failed; retrying.' >&2
    else
      show_patch clock "$clock_x" "$clock_y"
      rc=$?
      if [ "$rc" -eq 2 ]; then
        show_full || echo 'Date change redraw failed; retrying.' >&2
      elif [ "$rc" -ne 0 ]; then
        echo 'Clock update failed; retaining previous display.' >&2
      elif [ "$current_hour" != "$displayed_hour" ]; then
        show_patch weather "$weather_x" "$weather_y"
        rc=$?
        if [ "$rc" -eq 0 ]; then displayed_hour=$current_hour
        elif [ "$rc" -eq 2 ]; then show_full || echo 'Date change redraw failed.' >&2
        else echo 'Weather update failed; retrying next minute.' >&2; fi
      fi
    fi
  else
    echo 'Invalid server address.' >&2
  fi
  now=$(date +%s)
  delay=$((60 - now % 60))
  landscape_sleep "$delay"
done
