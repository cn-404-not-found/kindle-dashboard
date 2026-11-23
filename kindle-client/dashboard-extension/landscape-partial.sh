#!/bin/sh
# PW2/PW3 landscape online mode: full paint on startup/day change, partial paints otherwise.
DIR=${KINDLE_DASHBOARD_DIR:-/mnt/us/extensions/kindle-dashboard}
KINDLE_DASHBOARD_DIR=$DIR
TMP_DIR=${KINDLE_PARTIAL_STATE:-/tmp/kindle-landscape-partial}
mkdir -p "$TMP_DIR" || exit 1
. "$DIR/landscape-ui.sh" || exit 1
. "$DIR/network-health.sh" || exit 1

page=${KINDLE_DASHBOARD_PAGE:-home}
case "$page" in ''|*[!a-z0-9_-]*) page=home ;; esac
page_total=1
page_index=1
prev_page=
next_page=
last_full=0
page_request=$TMP_DIR/page-request
rm -f "$page_request"
swipe_pid=
cleanup_client() {
  [ -n "$swipe_pid" ] && kill "$swipe_pid" 2>/dev/null
  landscape_ui_cleanup
}
trap cleanup_client TERM INT HUP
trap 'page_requested=1' USR1

if [ "${KINDLE_PAGE_SWIPE:-1}" != 0 ]; then
  /bin/sh "$DIR/touch-swipe.sh" "${KINDLE_TOUCH_DEVICE:-/dev/input/event1}" "$TMP_DIR" "$$" \
    >> "${KINDLE_DASHBOARD_LOG:-/tmp/kindle-dashboard.log}" 2>&1 &
  swipe_pid=$!
fi

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
  # Older servers do not send the page header; they only serve the home page.
  server_page=$(header X-Dashboard-Page)
  if [ -n "$server_page" ]; then
    [ "$server_page" = "$page" ] || return 1
  elif [ "$page" != home ]; then
    return 1
  fi
  current_day=$(header X-Dashboard-Date)
  current_hour=$(header X-Dashboard-Hour)
  case "$current_day" in ????-??-??) ;; *) return 1 ;; esac
  case "$current_hour" in ??????????) ;; *) return 1 ;; esac
  case "$current_day$current_hour" in *[!0-9-]*) return 1 ;; esac
}
fetch() {
  part=$1
  query="?page=$page"
  [ -n "$part" ] && query="$query&part=$part"
  curl -fsS --connect-timeout 10 --max-time 45 -D "$TMP_DIR/headers" -o "$TMP_DIR/image.png" \
    -H "png-width: $width" -H "png-height: $height" -H 'X-Dashboard-Layout: landscape-v1' \
    "${url}/screen.png${query}" && [ -s "$TMP_DIR/image.png" ] && valid_metadata
}
read_page_nav() {
  page_total=$(header X-Dashboard-Pages)
  case "$page_total" in ''|*[!0-9]*) page_total=1 ;; esac
  page_index=$(header X-Dashboard-Page-Index)
  case "$page_index" in ''|*[!0-9]*) page_index=1 ;; esac
  next_page=$(header X-Dashboard-Next-Page)
  prev_page=$(header X-Dashboard-Prev-Page)
}
show_full() {
  if ! fetch ''; then echo 'Full image unavailable or server lacks partial support.' >&2; return 1; fi
  if ! landscape_ui_start; then echo 'Could not suppress status bar.' >&2; return 1; fi
  eips -c && landscape_sleep 1 && eips -g "$TMP_DIR/image.png" || return 1
  displayed_day=$current_day
  displayed_hour=$current_hour
  read_page_nav
  last_full=$(date +%s)
}

# show_quick 在两次整页刷新之间更新全部内容：不清屏，避免每分钟闪烁。
# 带 -x/-y 走局部更新路径；跨天时仍走 show_full，保证日期和残影一起重置。
show_quick() {
  fetch '' || return 1
  if [ "$current_day" != "$displayed_day" ]; then
    show_full || return 1
    return 0
  fi
  eips -g "$TMP_DIR/image.png" -x 0 -y 0
}

show_patch() {
  part=$1; expected_x=$2; expected_y=$3
  fetch "$part" || return 1
  [ "$(header X-Dashboard-Part)" = "$part" ] && [ "$(header X-Dashboard-X)" = "$expected_x" ] && [ "$(header X-Dashboard-Y)" = "$expected_y" ] || return 1
  [ "$current_day" = "$displayed_day" ] || return 2
  eips -g "$TMP_DIR/image.png" -x "$expected_x" -y "$expected_y"
}

displayed_day=
displayed_hour=
apply_page_request() {
  [ -f "$page_request" ] || return 0
  request=$(cat "$page_request" 2>/dev/null)
  rm -f "$page_request"
  case "$request" in next) target=$next_page ;; prev) target=$prev_page ;; *) return 0 ;; esac
  case "$target" in ''|*[!a-z0-9_-]*) return 0 ;; esac
  [ "$target" = "$page" ] && return 0
  previous=$page
  page=$target
  if show_full; then
    echo "Switched to page $page ($page_index/$page_total)." >&2
  else
    page=$previous
    echo "Page switch to $target failed; keeping $page." >&2
  fi
}
while :; do
  url=$(/bin/sh "$DIR/server-settings.sh" --get) || url=
  if [ -n "$url" ]; then
    if [ "${KINDLE_WIFI_WATCHDOG:-1}" != 0 ]; then
      # Keep the device awake and bring the radio back before touching curl.
      network_keep_awake
      network_ensure_online
    fi
    apply_page_request
    if [ -z "$displayed_day" ]; then
      show_full || echo 'Full display failed; retrying.' >&2
    elif [ "$page" = home ]; then
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
    else
      now=$(date +%s)
      if [ $((now - last_full)) -ge 300 ]; then
        show_full || echo 'Page refresh failed; retaining previous display.' >&2
      else
        show_quick || echo 'Page update failed; retaining previous display.' >&2
      fi
    fi
  else
    echo 'Invalid server address.' >&2
  fi
  now=$(date +%s)
  delay=$((60 - now % 60))
  landscape_sleep "$delay"
done
