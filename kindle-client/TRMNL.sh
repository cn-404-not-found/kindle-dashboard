#!/bin/sh
#
# TRMNL.sh — Kindle 墨水屏看板客户端（适配 kindle-dashboard 服务端）
#
# 功能：
#   1. 向服务端 GET /api/display 取图信息（image_url / refresh_rate）
#   2. 下载 PNG 图片
#   3. 用 eips 显示到墨水屏
#   4. 按 refresh_rate 循环刷新
#
# 说明：
#   - 本脚本【自包含】，不再依赖 utils.sh。
#   - 服务端只使用 png-width / png-height 两个请求头来按设备分辨率自适应出图。
#   - 其余头（access-token / battery-voltage / rssi / ID）服务端忽略，保留仅为兼容 trmnl 协议。
#

# ============================ 用户配置 ============================
# 在 KUAL 的 Set server address 中设置；每次刷新读取，不执行配置内容。
SERVER_SETTINGS="${KINDLE_DASHBOARD_DIR:-/mnt/us/extensions/kindle-dashboard}/server-settings.sh"

API_KEY="kindle-dashboard"        # 服务端不校验，随便填
USER_AGENT="trmnl-display/0.1.1"
DEBUG_MODE=false                  # true=屏幕上显示调试文字；false=正式使用（干净画面）
# ================================================================

TMP_DIR="/tmp/trmnl-kindle"
mkdir -p "$TMP_DIR"

# ---------------------- 设备信息（自带，无需 utils.sh） ----------------------
# 从 eips -i 读取 Kindle 帧缓冲真实分辨率
get_kindle_width() {
  eips -i 2>/dev/null | grep 'xres:' | head -1 | awk '{print $2}'
}
get_kindle_height() {
  eips -i 2>/dev/null | sed -n 's/.*yres:[ ]*\([0-9]*\).*/\1/p'
}
get_mac_address() {
  ifconfig wlan0 2>/dev/null | grep -i 'HWaddr' | awk '{print $5}'
}
get_kindle_battery() {
  echo "0"   # 服务端忽略，占位
}

PNG_WIDTH=$(get_kindle_width)
PNG_HEIGHT=$(get_kindle_height)

# 若 eips 不存在（本机测试环境），用桩函数避免报错
if ! command -v eips >/dev/null 2>&1; then
  eips() { echo "[eips STUB] $*"; }
fi

# 调试文字：仅 DEBUG_MODE=true 时打印到屏幕
DEBUG_Y=0
eips_debug() {
  [ "$DEBUG_MODE" = true ] || return 0
  eips 0 "$DEBUG_Y" "$1"
  echo "$1"
  DEBUG_Y=$((DEBUG_Y + 1))
}

# ----------------------------- 主循环 -----------------------------
while true; do
  DEBUG_Y=0
  BASE_URL=$(/bin/sh "$SERVER_SETTINGS" --get) || {
    eips_debug "Invalid server address."
    sleep 60
    continue
  }
  eips_debug "TRMNL Kindle client"

  # 1) 取图信息
  RESPONSE=$(curl -fsS --connect-timeout 10 --max-time 30 \
    -H "access-token: $API_KEY" \
    -H "battery-voltage: $(get_kindle_battery)" \
    -H "png-width: $PNG_WIDTH" \
    -H "png-height: $PNG_HEIGHT" \
    -H "rssi: 0" \
    -H "ID: $(get_mac_address)" \
    -A "$USER_AGENT" \
    "${BASE_URL}/api/display")

  if [ -z "$RESPONSE" ]; then
    eips_debug "Error: No JSON response."
    eips_debug "Retry in 60s..."
    sleep 60
    continue
  fi

  # 2) 解析 image_url（服务端返回 /screen.png 这类相对路径）
  IMAGE_URL=$(echo "$RESPONSE" | sed -n 's/.*"image_url":"\([^"]*\)".*/\1/p' | sed 's/\\u0026/\&/g')
  REFRESH_RATE=$(echo "$RESPONSE" | sed -n 's/.*"refresh_rate":\([^,}]*\).*/\1/p')
  [ -z "$REFRESH_RATE" ] && REFRESH_RATE="60"

  if [ -z "$IMAGE_URL" ]; then
    eips_debug "Error: parse image_url failed"
    eips_debug "Retry in 60s..."
    sleep 60
    continue
  fi
  eips_debug "URL: $IMAGE_URL"

  # 3) 相对路径补全为绝对 URL（/screen.png → http://IP:PORT/screen.png）
  case "$IMAGE_URL" in
    http://*|https://*) : ;;
    /*) IMAGE_URL="${BASE_URL}${IMAGE_URL}" ;;
    *)  IMAGE_URL="${BASE_URL}/$IMAGE_URL" ;;
  esac

  # 4) 下载图片
  FILENAME=$(echo "$IMAGE_URL" | sed -n 's#.*/\([^?/]*\)\?.*#\1#p')
  [ -z "$FILENAME" ] && FILENAME="dashboard.png"
  case "$FILENAME" in
    *.png) : ;;
    *) FILENAME="${FILENAME}.png" ;;
  esac
  IMAGE_PATH="$TMP_DIR/$FILENAME"
  rm -f "$IMAGE_PATH"

  if ! curl -fsS --connect-timeout 10 --max-time 45 -o "$IMAGE_PATH" \
    -H "png-width: $PNG_WIDTH" -H "png-height: $PNG_HEIGHT" -A "$USER_AGENT" "$IMAGE_URL" || [ ! -s "$IMAGE_PATH" ]; then
    eips_debug "Error: download failed!"
    eips_debug "Retry in 60s..."
    sleep 60
    continue
  fi

  # 5) 显示（始终先清屏，避免残影；eips -g 1:1 铺满）
  eips -c
  sleep 1
  eips -g "$IMAGE_PATH"

  eips_debug "Sleeping for ${REFRESH_RATE}s..."
  sleep "$REFRESH_RATE"
done
