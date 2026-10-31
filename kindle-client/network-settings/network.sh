#!/bin/sh
# Interactive, manually applied IPv4 configuration for jailbroken Kindle.
# Uses no eval/source for values from the writable USB partition.
PATH=/usr/sbin:/usr/bin:/sbin:/bin
export PATH
CONFIG_DIR=${KINDLE_NET_CONFIG_DIR:-/mnt/us/extensions/kindle-network}
CONFIG="$CONFIG_DIR/network.conf"

valid_ip() {
    printf '%s\n' "$1" | awk -F. 'NF != 4 {exit 1} {for(i=1;i<=4;i++) if ($i !~ /^[0-9]+$/ || length($i)>3 || (length($i)>1 && substr($i,1,1)=="0") || $i>255) exit 1; if ($1==0 || $1>=224) exit 1}'
}
valid_mask() {
    printf '%s\n' "$1" | awk -F. 'NF != 4 {exit 1} {end=0; for(i=1;i<=4;i++){v=$i; if(v !~ /^[0-9]+$/ || (length(v)>1 && substr(v,1,1)=="0")) exit 1; if(end && v!=0) exit 1; if(v!=255){if(v!=254 && v!=252 && v!=248 && v!=240 && v!=224 && v!=192 && v!=128 && v!=0) exit 1; end=1}} if($1==0 || ($1==255 && $2==255 && $3==255 && $4==255)) exit 1}'
}
valid_subnet() {
    awk -v a="$1" -v m="$2" -v g="$3" 'BEGIN {
        split(a,aa,"."); split(m,mm,"."); split(g,gg,".");
        ip=0; mask=0; gateway=0;
        for(i=1;i<=4;i++){ip=ip*256+aa[i]; mask=mask*256+mm[i]; gateway=gateway*256+gg[i]}
        size=4294967296-mask;
        if(size<4 || int(ip/size)!=int(gateway/size) || ip==gateway || ip%size==0 || ip%size==size-1 || gateway%size==0 || gateway%size==size-1) exit 1;
    }'
}
validate() { valid_ip "$1" && valid_mask "$2" && valid_ip "$3" && valid_ip "$4" && valid_subnet "$1" "$2" "$3"; }
valid_target() {
    case "$1" in ''|-*|*[!A-Za-z0-9.-]*) return 1 ;; esac
    [ "${#1}" -le 253 ] || return 1
    return 0
}
if [ "${1:-}" = "--validate-target" ]; then
    [ "$#" -eq 2 ] && valid_target "$2"
    exit $?
fi
if [ "${1:-}" = "--validate" ]; then
    [ "$#" -eq 5 ] && validate "$2" "$3" "$4" "$5"
    exit $?
fi

# Restore the network snapshot made by a timed test; never source writable files.
rollback() {
    backup=$1
    [ -f "$backup/settings" ] && [ -f "$backup/resolv.conf" ] || return 1
    { IFS= read -r old_addr && IFS= read -r old_mask && IFS= read -r old_gateway && IFS= read -r new_gateway; } < "$backup/settings" || return 1
    valid_ip "$new_gateway" || return 1
    if [ "$old_addr" = none ]; then
        ifconfig wlan0 0.0.0.0 || return 1
    else
        valid_ip "$old_addr" && valid_mask "$old_mask" || return 1
        ifconfig wlan0 "$old_addr" netmask "$old_mask" up || return 1
    fi
    if [ "$old_gateway" = none ]; then
        ip route del default via "$new_gateway" dev wlan0 2>/dev/null || :
    else
        valid_ip "$old_gateway" || return 1
        ip route replace default via "$old_gateway" dev wlan0 || return 1
    fi
    cat "$backup/resolv.conf" > /var/run/resolv.conf || return 1
}
if [ "${1:-}" = "--rollback" ]; then
    rollback "$2"
    exit $?
fi

# /proc/mounts may list an overmounted rootfs before the real root filesystem.
root_mount_mode() {
    awk '$2 == "/" { options=$4 } END { n=split(options,a,","); for(i=1;i<=n;i++) { if(a[i]=="ro" || a[i]=="rw") {print a[i]; exit} } exit 1 }'
}
if [ "${1:-}" = "--root-mount-mode" ]; then
    root_mount_mode
    exit $?
fi

pause() { printf '\nPress Enter to continue... '; read -r answer; }
get_field() {
    while :; do
        printf '%s: ' "$1"; IFS= read -r value || return 1
        if "$2" "$value"; then return 0; fi
        printf 'Invalid value; please try again.\n'
    done
}
load_config() {
    [ -f "$CONFIG" ] || { echo 'No saved settings.'; return 1; }
    { IFS= read -r addr && IFS= read -r mask && IFS= read -r gateway && IFS= read -r dns; } < "$CONFIG" || return 1
    validate "$addr" "$mask" "$gateway" "$dns" || { echo 'Saved settings are invalid.'; return 1; }
}
show_config() { printf 'IP:      %s\nMask:    %s\nGateway: %s\nDNS:     %s\n' "$addr" "$mask" "$gateway" "$dns"; }
save_config() {
    echo 'Enter IPv4 settings for the target Wi-Fi network.'
    get_field 'Kindle IP' valid_ip || return
    addr=$value
    get_field 'Subnet mask' valid_mask || return
    mask=$value
    get_field 'Gateway' valid_ip || return
    gateway=$value
    get_field 'DNS server' valid_ip || return
    dns=$value
    if ! validate "$addr" "$mask" "$gateway" "$dns"; then
        echo 'Invalid settings: check subnet, IP and gateway.'; return 1
    fi
    printf '\n'; show_config
    printf '\nSave these settings? [y/N]: '; read -r answer
    [ "$answer" = y ] || [ "$answer" = Y ] || { echo 'Cancelled.'; return; }
    mkdir -p "$CONFIG_DIR" || { echo 'Cannot create settings directory.'; return 1; }
    tmp="$CONFIG.tmp.$$"
    (umask 077; printf '%s\n%s\n%s\n%s\n' "$addr" "$mask" "$gateway" "$dns" > "$tmp") && mv "$tmp" "$CONFIG" || { rm -f "$tmp"; echo 'Save failed.'; return 1; }
    echo 'Saved. Network has not been changed.'
}
apply_config() {
    load_config || return 1
    echo 'Settings to apply to wlan0:'; show_config
    echo 'This changes the active network and may disconnect Wi-Fi.'
    echo 'Reconnect Wi-Fi or restart Kindle to restore automatic settings.'
    printf 'Apply now? [y/N]: '; read -r answer
    [ "$answer" = y ] || [ "$answer" = Y ] || { echo 'Cancelled.'; return; }
    # Require an existing association; avoid switching Wi-Fi profiles or credentials.
    wpa_cli -i wlan0 status 2>/dev/null | grep -q '^wpa_state=COMPLETED$' || { echo 'Wi-Fi is not associated. Connect to the target network first.'; return 1; }
    # Snapshot only the existing wlan0 address, gateway and resolver before testing.
    current=$(ifconfig wlan0 2>/dev/null)
    old_addr=$(printf '%s\n' "$current" | sed -n 's/.*inet addr:\([0-9.]*\).*/\1/p' | head -1)
    old_mask=$(printf '%s\n' "$current" | sed -n 's/.*Mask:\([0-9.]*\).*/\1/p' | head -1)
    old_gateway=$(route -n 2>/dev/null | awk '$1=="0.0.0.0" && $NF=="wlan0" {print $2; exit}')
    if [ -z "$old_addr" ]; then
        old_addr=none; old_mask=none
    elif ! valid_ip "$old_addr" || ! valid_mask "$old_mask"; then
        echo 'Cannot snapshot current address. Test aborted.'; return 1
    fi
    if [ -z "$old_gateway" ]; then old_gateway=none
    elif ! valid_ip "$old_gateway"; then echo 'Cannot snapshot current gateway.'; return 1
    fi
    backup="/tmp/kindle-network-rollback.$$"
    (umask 077; mkdir "$backup") || { echo 'Cannot create rollback snapshot.'; return 1; }
    printf '%s\n%s\n%s\n%s\n' "$old_addr" "$old_mask" "$old_gateway" "$gateway" > "$backup/settings"
    cp /var/run/resolv.conf "$backup/resolv.conf" || { echo 'Cannot snapshot DNS.'; return 1; }
    # A detached watchdog restores the original network unless confirmed in time.
    (trap '' HUP; sleep 90; /bin/sh "$0" --rollback "$backup"; echo done > "$backup/expired") &
    watchdog=$!
    if ! ifconfig wlan0 "$addr" netmask "$mask" up ||
       ! ip route replace default via "$gateway" dev wlan0 ||
       ! printf 'nameserver %s\n' "$dns" > /var/run/resolv.conf; then
        kill "$watchdog" 2>/dev/null
        rollback "$backup" || echo 'Restore failed; restart Kindle.'
        echo 'Apply failed; original settings restored if possible.'; return 1
    fi
    echo 'Temporary test applied for 90 seconds.'
    ifconfig wlan0
    route -n
    echo 'Check connectivity. Enter y within 90 seconds to keep.'
    echo 'Otherwise the previous settings will be restored.'
    printf 'Keep changes? [y/N]: '; IFS= read -r answer
    if { [ "$answer" = y ] || [ "$answer" = Y ]; } && [ ! -e "$backup/expired" ] && kill -0 "$watchdog" 2>/dev/null; then
        kill "$watchdog" 2>/dev/null
        echo 'Kept for this session. Wi-Fi manager may overwrite it.'
        echo 'It does not automatically apply after reboot.'
    else
        kill "$watchdog" 2>/dev/null
        rollback "$backup" || { echo 'Restore failed; restart Kindle.'; return 1; }
        echo 'Original network settings restored.'
    fi

}
# Install one uniquely marked cron line; preserve all existing entries.
install_watchdog() {
    cron=/etc/crontab/root
    marker='kindle-network-watchdog'
    [ -f "$cron" ] || { echo 'Cron configuration not found.'; return 1; }
    grep -Fq "$marker" "$cron" && return 0
    root_mode=$(root_mount_mode < /proc/mounts)
    case "$root_mode" in ro) was_ro=1 ;; rw) was_ro=0 ;; *) echo 'Cannot determine root mount mode.'; return 1 ;; esac
    if [ "$was_ro" = 1 ]; then
        mntroot rw || { echo 'Unable to remount root read-write.'; return 1; }
        if [ "$(root_mount_mode < /proc/mounts)" != rw ]; then
            mntroot ro
            echo 'Root filesystem is still read-only.'; return 1
        fi
    fi
    printf '\n* * * * * /bin/sh /mnt/us/extensions/kindle-network/auto.sh --ensure >/dev/null 2>&1 # kindle-network-watchdog\n' >> "$cron"
    result=$?
    if [ "$was_ro" = 1 ]; then mntroot ro || { echo 'Could not restore root read-only mode.'; return 1; }; fi
    [ "$result" -eq 0 ] || { echo 'Could not install startup entry.'; return 1; }
    kill -HUP "$(pidof crond)" 2>/dev/null || echo 'Cron reload failed; restart may be required.'
}
static_mode() {
    load_config || return 1
    status=$(wpa_cli -i wlan0 status 2>/dev/null)
    printf '%s\n' "$status" | grep -q '^wpa_state=COMPLETED$' || { echo 'Connect to the target Wi-Fi first.'; return 1; }
    ssid=$(printf '%s\n' "$status" | sed -n 's/^ssid=//p' | head -1)
    [ -n "$ssid" ] || { echo 'Could not read current Wi-Fi name.'; return 1; }
    echo "Static mode for Wi-Fi: $ssid"; show_config
    echo 'Reapplies every 5 seconds on this Wi-Fi, including after reboot.'
    echo 'Other Wi-Fi networks will not use these settings.'
    printf 'Enable static mode? [y/N]: '; read -r answer
    [ "$answer" = y ] || [ "$answer" = Y ] || return 0
    install_watchdog || { echo 'Startup installation failed; mode unchanged.'; return 1; }
    printf '%s\n' "$ssid" > "$CONFIG_DIR/ssid" || return 1
    printf 'static\n' > "$CONFIG_DIR/mode" || return 1
    /bin/sh "$CONFIG_DIR/auto.sh" --ensure
    echo 'Static mode enabled. Verify Wi-Fi and connectivity.'
}
dhcp_mode() {
    echo 'Automatic mode will disable static reapply and reconnect Wi-Fi.'
    printf 'Switch to automatic DHCP? [y/N]: '; read -r answer
    [ "$answer" = y ] || [ "$answer" = Y ] || return 0
    mkdir -p "$CONFIG_DIR" || return 1
    printf 'dhcp\n' > "$CONFIG_DIR/mode" || return 1
    if ! lipc-set-prop com.lab126.cmd wirelessEnable 0; then echo 'Mode saved; turn Wi-Fi off and on manually.'; return 1; fi
    sleep 2
    if ! lipc-set-prop com.lab126.cmd wirelessEnable 1; then echo 'Turn Wi-Fi on manually in Settings.'; return 1; fi
    echo 'Automatic mode selected. Wait for Wi-Fi to reconnect.'
}
ping_target() {
    printf 'Host or IP to ping: '; IFS= read -r target || return 1
    valid_target "$target" || { echo 'Invalid host or IP.'; return 1; }
    echo "Testing $target (3 packets)..."
    ping -c 3 -W 3 "$target"
    result=$?
    if [ "$result" -eq 0 ]; then echo 'Ping succeeded.'; else echo 'Ping failed; the host may block ping or DNS may not work.'; fi
    return "$result"
}
while :; do
    clear
    echo 'Kindle Wi-Fi IPv4 Settings'
    mode=$(cat "$CONFIG_DIR/mode" 2>/dev/null)
    if [ "$mode" = static ]; then echo "Mode: STATIC ($(cat "$CONFIG_DIR/ssid" 2>/dev/null))"; else echo 'Mode: AUTO (DHCP)'; fi
    echo
    echo '1. Enter and save settings'
    echo '2. View saved settings'
    echo '3. Apply saved settings now'
    echo '4. Show current network'
    echo '5. Ping host or IP'
    echo '6. Use static IP (automatic reapply)'
    echo '7. Use automatic IP (DHCP)'
    echo '8. Exit'
    printf '\nChoice: '; IFS= read -r choice || exit 0
    case "$choice" in
        1) save_config; pause ;;
        2) load_config && show_config; pause ;;
        3) apply_config; pause ;;
        4) ifconfig wlan0; route -n; pause ;;
        5) ping_target; pause ;;
        6) static_mode; pause ;;
        7) dhcp_mode; pause ;;
        8) exit 0 ;;
        *) echo 'Choose 1-8.'; pause ;;
    esac
done
