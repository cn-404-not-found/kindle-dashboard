#!/bin/sh
# Server address is plain text, never executed or sourced as shell code.
CONFIG_DIR=${KINDLE_DASHBOARD_DIR:-/mnt/us/extensions/kindle-dashboard}
CONFIG="$CONFIG_DIR/server-url.txt"
DEFAULT_URL='http://192.168.100.231:58188'
normalize() {
    value=$1
    case "$value" in http://*) scheme=http; authority=${value#http://} ;; https://*) scheme=https; authority=${value#https://} ;; *://*) return 1 ;; *) scheme=http; authority=$value ;; esac
    case "$authority" in */) authority=${authority%/} ;; esac
    case "$authority" in ''|*[!A-Za-z0-9.:-]*|*:*:*) return 1 ;; esac
    case "$authority" in *:*) host=${authority%:*}; port=${authority##*:}; case "$port" in ''|*[!0-9]*) return 1 ;; esac; [ "${#port}" -le 5 ] && [ "$port" -gt 0 ] && [ "$port" -le 65535 ] || return 1 ;; *) host=$authority ;; esac
    printf '%s\n' "$host" | awk '
      length($0)<1 || length($0)>253 {exit 1}
      { n=split($0, labels, "."); for(i=1;i<=n;i++) if(length(labels[i])<1 || length(labels[i])>63 || labels[i] !~ /^[A-Za-z0-9-]+$/ || labels[i] ~ /^-/ || labels[i] ~ /-$/) exit 1;
        if ($0 ~ /^[0-9.]+$/) {if(n!=4) exit 1; for(i=1;i<=4;i++) if(labels[i]>255 || (length(labels[i])>1 && substr(labels[i],1,1)=="0")) exit 1} }
    ' || return 1
    printf '%s://%s\n' "$scheme" "$authority"
}
get_url() {
    if [ -f "$CONFIG" ]; then
        IFS= read -r address < "$CONFIG" || return 1
        normalize "$address"
    else
        printf '%s\n' "$DEFAULT_URL"
    fi
}
case "${1:-}" in
    --normalize) [ "$#" -eq 2 ] && normalize "$2"; exit $? ;;
    --get) get_url; exit $? ;;
esac
clear
printf 'Dashboard server address\n\n'
current=$(get_url) || current='(invalid configuration)'
printf 'Current: %s\n\n' "$current"
printf 'Enter host:port or full http(s) URL.\nExample: 192.168.10.103:8080\nLeave blank to cancel.\n\nAddress: '
IFS= read -r address || exit 0
[ -n "$address" ] || { echo 'Cancelled.'; sleep 1; exit 0; }
normalized=$(normalize "$address") || { printf '\nInvalid address. Nothing saved.\nPress Enter to exit.'; read -r ignored; exit 1; }
printf '\nSave %s ? [y/N]: ' "$normalized"
IFS= read -r answer || exit 0
case "$answer" in y|Y) ;; *) echo 'Cancelled.'; sleep 1; exit 0 ;; esac
mkdir -p "$CONFIG_DIR" || { echo 'Could not create configuration directory.'; exit 1; }
tmp="$CONFIG.tmp.$$"
(umask 077; printf '%s\n' "$normalized" > "$tmp") && mv "$tmp" "$CONFIG" || { rm -f "$tmp"; echo 'Save failed.'; exit 1; }
printf '\nSaved. Running dashboard uses the new address\non its next refresh (about 60 seconds).\nPress Enter to exit.'
read -r ignored
