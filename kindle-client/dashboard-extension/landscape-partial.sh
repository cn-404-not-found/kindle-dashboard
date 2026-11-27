#!/bin/sh
# PW2/PW3 landscape online mode: full paint on startup/day change, partial paints otherwise.
DIR=${KINDLE_DASHBOARD_DIR:-/mnt/us/extensions/kindle-dashboard}
KINDLE_DASHBOARD_DIR=$DIR
KINDLE_WIFI_EVENT_LOG=${KINDLE_WIFI_EVENT_LOG:-$DIR/wifi-events.log}
export KINDLE_WIFI_EVENT_LOG
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
usage_revision=
displayed_revision=
last_full=0
article_id=
article_page=1
article_total=1
article_activity=0
article_timeout=${KINDLE_ARTICLE_TIMEOUT:-180}
case "$article_timeout" in ''|*[!0-9]*) article_timeout=180 ;; esac
next_refresh=0
request_handled=0
fetch_fail_streak=0
page_request=$TMP_DIR/page-request
rm -f "$page_request"
swipe_pid=
cleanup_client() {
  [ -n "$swipe_pid" ] && kill "$swipe_pid" 2>/dev/null
  landscape_ui_cleanup
}
trap cleanup_client TERM INT HUP
page_requested=0
trap 'page_requested=1; [ -n "${landscape_sleep_pid:-}" ] && kill "$landscape_sleep_pid" 2>/dev/null' USR1

if [ "${KINDLE_PAGE_SWIPE:-1}" != 0 ]; then
  /bin/sh "$DIR/touch-swipe.sh" "${KINDLE_TOUCH_DEVICE:-/dev/input/event1}" "$TMP_DIR" "$$" \
    >> "${KINDLE_DASHBOARD_LOG:-/tmp/kindle-dashboard.log}" 2>&1 &
  swipe_pid=$!
fi

width=$(eips -i 2>/dev/null | sed -n 's/.*xres:[ ]*\([0-9]*\).*/\1/p' | head -1)
height=$(eips -i 2>/dev/null | sed -n 's/.*yres:[ ]*\([0-9]*\).*/\1/p' | head -1)
case "$width:$height" in
  1072:1448) clock_x=852; clock_y=55; weather_x=136; weather_y=55; news_x=36; news_y=680; pageheader_x=948; pageheader_y=620 ;;
  758:1024) clock_x=602; clock_y=38; weather_x=96; weather_y=38; news_x=25; news_y=480; pageheader_x=670; pageheader_y=438 ;;
  *) echo "Unsupported framebuffer: $width x $height" >&2; exit 1 ;;
esac
touch_max_x=${KINDLE_TOUCH_MAX_X:-$width}
touch_max_y=${KINDLE_TOUCH_MAX_Y:-$height}
case "$touch_max_x:$touch_max_y" in ''|*[!0-9:]*|:*|*:) touch_max_x=$width; touch_max_y=$height ;; esac

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
  usage_revision=$(header X-Dashboard-Usage-Revision)
  case "$usage_revision" in ''|*[!a-f0-9]*) usage_revision= ;; esac
}
fetch() {
  part=$1
  query="?page=$page"
  if [ "$page" = news_detail ] && [ -n "$article_id" ]; then
    query="${query}&id=${article_id}&p=${article_page}"
  fi
  [ -n "$part" ] && query="$query&part=$part"
  if curl -fsS --connect-timeout 10 --max-time 45 -D "$TMP_DIR/headers" -o "$TMP_DIR/image.png" \
    -H "png-width: $width" -H "png-height: $height" -H 'X-Dashboard-Layout: landscape-v1' \
    "${url}/screen.png${query}" && [ -s "$TMP_DIR/image.png" ] && valid_metadata; then
    fetch_fail_streak=0
    return 0
  fi
  fetch_fail_streak=$((fetch_fail_streak + 1))
  return 1
}
read_page_nav() {
  page_total=$(header X-Dashboard-Pages)
  case "$page_total" in ''|*[!0-9]*) page_total=1 ;; esac
  page_index=$(header X-Dashboard-Page-Index)
  case "$page_index" in ''|*[!0-9]*) page_index=1 ;; esac
  next_page=$(header X-Dashboard-Next-Page)
  prev_page=$(header X-Dashboard-Prev-Page)
  if [ "$page" = news_detail ]; then
    hdr_id=$(header X-Dashboard-Article-Id)
    case "$hdr_id" in ''|*[!a-zA-Z0-9_-]*) ;; *) article_id=$hdr_id ;; esac
    hdr_page=$(header X-Dashboard-Article-Page)
    case "$hdr_page" in ''|*[!0-9]*) hdr_page=1 ;; esac
    hdr_total=$(header X-Dashboard-Article-Total)
    case "$hdr_total" in ''|*[!0-9]*) hdr_total=1 ;; esac
    article_page=$hdr_page
    article_total=$hdr_total
    article_activity=$(date +%s)
  fi
}
show_full() {
  if ! fetch ''; then echo 'Full image unavailable or server lacks partial support.' >&2; return 1; fi
  if ! landscape_ui_start; then echo 'Could not suppress status bar.' >&2; return 1; fi
  eips -c && landscape_sleep 1 && eips -g "$TMP_DIR/image.png" || return 1
  displayed_day=$current_day
  displayed_hour=$current_hour
  displayed_revision=$usage_revision
  read_page_nav
  last_full=$(date +%s)
  next_refresh=$((last_full + 60 - last_full % 60))
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
  displayed_revision=$usage_revision
}

show_patch() {
  part=$1; expected_x=$2; expected_y=$3
  fetch "$part" || return 1
  [ "$(header X-Dashboard-Part)" = "$part" ] && [ "$(header X-Dashboard-X)" = "$expected_x" ] && [ "$(header X-Dashboard-Y)" = "$expected_y" ] || return 1
  [ "$current_day" = "$displayed_day" ] || return 2
  eips -g "$TMP_DIR/image.png" -x "$expected_x" -y "$expected_y"
}

# show_header_patch 先取页头并判断用量版本：内容没变只画页头；
# 变了则跳过页头，直接整页快速刷新，避免同一分钟画两次。
show_header_patch() {
  fetch pageheader || return 1
  [ "$(header X-Dashboard-Part)" = pageheader ] && [ "$(header X-Dashboard-X)" = "$pageheader_x" ] && [ "$(header X-Dashboard-Y)" = "$pageheader_y" ] || return 1
  [ "$current_day" = "$displayed_day" ] || return 2
  if [ -z "$usage_revision" ] || [ "$usage_revision" != "$displayed_revision" ]; then
    show_quick || return 1
    return 0
  fi
  eips -g "$TMP_DIR/image.png" -x "$pageheader_x" -y "$pageheader_y"
}

displayed_day=
displayed_hour=
open_article() {
  target_id=$1; target_page=$2
  case "$target_id" in ''|*[!a-zA-Z0-9_-]*) return 1 ;; esac
  case "$target_page" in ''|*[!0-9]*) target_page=1 ;; esac
  same_article=0
  [ "$page" = news_detail ] && [ "$article_id" = "$target_id" ] && same_article=1
  prev_page_state=$page; prev_id=$article_id; prev_ap=$article_page; prev_at=$article_total
  page=news_detail; article_id=$target_id; article_page=$target_page
  if [ "$same_article" -eq 1 ]; then
    if fetch '' && eips -g "$TMP_DIR/image.png" -x 0 -y 0; then
      displayed_day=$current_day
      displayed_hour=$current_hour
      read_page_nav
      return 0
    fi
  else
    if show_full; then
      return 0
    fi
  fi
  page=$prev_page_state; article_id=$prev_id; article_page=$prev_ap; article_total=$prev_at
  echo "Failed to open article $target_id page $target_page." >&2
  return 1
}

return_home() {
  [ "$page" = home ] && return 0
  prev_page_state=$page
  page=home
  if show_full; then
    article_id=
    article_page=1
    article_total=1
    echo "Returned to home." >&2
  else
    page=$prev_page_state
    echo "Return to home failed; keeping $page." >&2
  fi
}

resolve_tap() {
  raw_x=$1; raw_y=$2
  case "$raw_x:$raw_y" in ''|*[!0-9:]*|:*|*:) return 0 ;; esac
  tap_url="${url}/api/tap?page=${page}&x=${raw_x}&y=${raw_y}&w=${touch_max_x}&h=${touch_max_y}"
  if [ "$page" = news_detail ]; then
    tap_url="${tap_url}&id=${article_id}&p=${article_page}&total=${article_total}"
  fi
  resp=$(curl -fsS --connect-timeout 5 --max-time 10 "$tap_url" 2>/dev/null) || return 0
  set -- $resp
  case "$1" in
    open) open_article "$2" "${3:-1}" ;;
    home) return_home ;;
    *) [ "$page" = news_detail ] && article_activity=$(date +%s) ;;
  esac
}

apply_page_request() {
  page_requested=0
  request_handled=0
  [ -f "$page_request" ] || return 0
  request=$(cat "$page_request" 2>/dev/null)
  rm -f "$page_request"
  request_handled=1
  set -- $request
  case "$1" in
    tap)
      resolve_tap "$2" "$3"
      return 0
      ;;
    next|prev) ;;
    *) return 0 ;;
  esac
  if [ "$page" = news_detail ]; then
    if [ "$1" = next ]; then
      if [ "$article_page" -lt "$article_total" ]; then
        open_article "$article_id" $((article_page + 1))
      else
        article_activity=$(date +%s)
      fi
    else
      if [ "$article_page" -gt 1 ]; then
        open_article "$article_id" $((article_page - 1))
      else
        return_home
      fi
    fi
    return 0
  fi
  case "$1" in next) target=$next_page ;; prev) target=$prev_page ;; *) return 0 ;; esac
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
    now=$(date +%s)
    if [ "$page" = news_detail ]; then
      if [ $((now - article_activity)) -ge "$article_timeout" ]; then
        return_home
      fi
    elif [ -z "$displayed_day" ]; then
      show_full || echo 'Full display failed; retrying.' >&2
    elif [ "$request_handled" -eq 0 ] || [ "$now" -ge "$next_refresh" ]; then
      if [ "$page" = home ] || [ "$page" = calendar ]; then
        show_patch clock "$clock_x" "$clock_y"
        rc=$?
        if [ "$rc" -eq 2 ]; then
          show_full || echo 'Date change redraw failed; retrying.' >&2
        elif [ "$rc" -ne 0 ]; then
          echo 'Clock update failed; retaining previous display.' >&2
        else
          full_done=0
          if [ "$current_hour" != "$displayed_hour" ]; then
            show_patch weather "$weather_x" "$weather_y"
            rc=$?
            if [ "$rc" -eq 0 ]; then displayed_hour=$current_hour
            elif [ "$rc" -eq 2 ]; then
              show_full || echo 'Date change redraw failed.' >&2
              full_done=1
            else echo 'Weather update failed; retrying next minute.' >&2; fi
          fi
          if [ "$full_done" -eq 0 ] && [ "$page" = home ] && [ -n "$usage_revision" ] && [ "$usage_revision" != "$displayed_revision" ]; then
            show_patch news "$news_x" "$news_y"
            rc=$?
            if [ "$rc" -eq 0 ]; then displayed_revision=$usage_revision
            elif [ "$rc" -eq 2 ]; then show_full || echo 'Date change redraw failed.' >&2
            else echo 'News update failed; retrying next minute.' >&2; fi
          fi
        fi
      else
        now=$(date +%s)
        if [ $((now - last_full)) -ge 300 ]; then
          show_full || echo 'Page refresh failed; retaining previous display.' >&2
        else
          show_header_patch
          rc=$?
          if [ "$rc" -eq 2 ]; then
            show_full || echo 'Date change redraw failed; retrying.' >&2
          elif [ "$rc" -ne 0 ]; then
            echo 'Page header update failed; retaining previous display.' >&2
          fi
        fi
      fi
    fi
    if [ "${KINDLE_WIFI_WATCHDOG:-1}" != 0 ] && [ "$fetch_fail_streak" -ge 3 ] && [ $((fetch_fail_streak % 3)) -eq 0 ] && command -v network_force_recover >/dev/null 2>&1; then
      network_force_recover "$fetch_fail_streak"
    fi
  else
    echo 'Invalid server address.' >&2
  fi
  now=$(date +%s)
  if [ "$page" = news_detail ]; then
    remain=$((article_activity + article_timeout - now))
    if [ "$remain" -le 0 ]; then
      delay=30
    else
      delay=$remain
      [ "$delay" -gt 30 ] && delay=30
    fi
  else
    delay=$((60 - now % 60))
    next_refresh=$((now + delay))
  fi
  if [ "${page_requested:-0}" -eq 0 ] && [ ! -f "$page_request" ]; then
    landscape_sleep "$delay"
  fi
done
