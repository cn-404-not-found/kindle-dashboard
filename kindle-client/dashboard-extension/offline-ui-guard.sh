#!/bin/sh
# Independent recovery: if the dashboard dies, resume the window manager.
owner=$1
wm=$2
case "$owner:$wm" in *[!0-9:]*|:*|*:) exit 1 ;; esac
while kill -0 "$owner" 2>/dev/null; do sleep 5; done
kill -CONT "$wm" 2>/dev/null
