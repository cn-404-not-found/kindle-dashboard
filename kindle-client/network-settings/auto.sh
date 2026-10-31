#!/bin/sh
# Reapply only to the SSID selected by the user, never to other networks.
PATH=${KINDLE_NET_PATH:-/usr/sbin:/usr/bin:/sbin:/bin}; export PATH
DIR=${KINDLE_NET_CONFIG_DIR:-/mnt/us/extensions/kindle-network}
CONF="$DIR/network.conf"
MODE="$DIR/mode"
SSIDFILE="$DIR/ssid"
LOCK=/tmp/kindle-network-auto.lock
SCRIPT="$DIR/auto.sh"
RESOLV=${KINDLE_NET_RESOLV_PATH:-/var/run/resolv.conf}
active() { [ -f "$MODE" ] && [ "$(cat "$MODE")" = static ] && [ -f "$SSIDFILE" ] && [ -f "$CONF" ]; }
tick() {
    active || return 0
    expected=$(cat "$SSIDFILE")
    [ -n "$expected" ] || return 1
    status=$(wpa_cli -i wlan0 status 2>/dev/null)
    printf '%s\n' "$status" | grep -q '^wpa_state=COMPLETED$' || return 0
    current=$(printf '%s\n' "$status" | sed -n 's/^ssid=//p' | head -1)
    [ "$current" = "$expected" ] || return 0
    { IFS= read -r addr && IFS= read -r mask && IFS= read -r gateway && IFS= read -r dns; } < "$CONF" || return 1
    /bin/sh "$DIR/network.sh" --validate "$addr" "$mask" "$gateway" "$dns" || return 1
    current_if=$(ifconfig wlan0 2>/dev/null)
    if ! printf '%s\n' "$current_if" | grep -Fq "inet addr:$addr " || ! printf '%s\n' "$current_if" | grep -Fq "Mask:$mask"; then
        ifconfig wlan0 "$addr" netmask "$mask" up || return 1
    fi
    actual_gateway=$(route -n 2>/dev/null | awk '$1=="0.0.0.0" && $NF=="wlan0" {print $2; exit}')
    [ "$actual_gateway" = "$gateway" ] || ip route replace default via "$gateway" dev wlan0 || return 1
    grep -Fxq "nameserver $dns" "$RESOLV" 2>/dev/null || printf 'nameserver %s\n' "$dns" > "$RESOLV" || return 1
}
case "${1:-}" in
  --tick) tick ;;
  --ensure)
    active || exit 0
    if [ -d "$LOCK" ]; then
        pid=$(cat "$LOCK/pid" 2>/dev/null)
        case "$pid" in ''|*[!0-9]*) ;; *) kill -0 "$pid" 2>/dev/null && exit 0 ;; esac
        rm -f "$LOCK/pid"; rmdir "$LOCK" 2>/dev/null || exit 0
    fi
    mkdir "$LOCK" 2>/dev/null || exit 0
    (trap '' HUP; exec /bin/sh "$SCRIPT" --loop) </dev/null >/dev/null 2>&1 &
    echo $! > "$LOCK/pid"
    ;;
  --loop)
    trap 'rm -f "$LOCK/pid"; rmdir "$LOCK" 2>/dev/null; exit' EXIT TERM INT
    while active; do tick; sleep 5; done
    ;;
  *) exit 2 ;;
esac
