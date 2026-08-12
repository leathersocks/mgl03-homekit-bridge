#!/bin/sh
set -u

PID_FILE=/data/mgl03-homekit/bridge.pid
BRIDGE_BIN=/data/mgl03-homekit-bridge

bridge_pid_is_running() {
    PID=$1
    [ -n "$PID" ] || return 1
    [ -r "/proc/$PID/cmdline" ] || return 1
    CMDLINE=$(tr '\000' ' ' < "/proc/$PID/cmdline")
    case "$CMDLINE" in
        "$BRIDGE_BIN "*|"$BRIDGE_BIN") kill -0 "$PID" 2>/dev/null ;;
        *) return 1 ;;
    esac
}

if [ ! -f "$PID_FILE" ]; then
    echo "mgl03-homekit-bridge is not running"
    exit 0
fi

PID=$(cat "$PID_FILE")
if bridge_pid_is_running "$PID"; then
    kill "$PID"
    WAIT=0
    while [ "$WAIT" -lt 10 ] && bridge_pid_is_running "$PID"; do
        sleep 1
        WAIT=$((WAIT + 1))
    done
    if bridge_pid_is_running "$PID"; then
        echo "mgl03-homekit-bridge did not stop within 10 seconds (pid $PID)" >&2
        exit 1
    fi
    echo "stopped mgl03-homekit-bridge (pid $PID)"
else
    echo "removed stale or mismatched pid file"
fi
rm -f "$PID_FILE"
