#!/bin/sh
set -u
umask 077

STATE_DIR=/data/mgl03-homekit
BRIDGE_BIN=/data/mgl03-homekit-bridge
OPENMIIO_BIN=/data/openmiio_agent
CONFIG_FILE="$STATE_DIR/config.json"
LOG_FILE="$STATE_DIR/bridge.log"
PID_FILE="$STATE_DIR/bridge.pid"

openmiio_is_running() {
    for PID in $(pidof openmiio_agent 2>/dev/null); do
        if [ -r "/proc/$PID/cmdline" ]; then
            CMDLINE=$(tr '\000' ' ' < "/proc/$PID/cmdline")
            case "$CMDLINE" in
                *miio*central*mqtt*cache*|*miio*mqtt*cache*central*) return 0 ;;
            esac
        fi
    done
    return 1
}

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

mqtt_is_ready() {
    netstat -lnt 2>/dev/null | grep -e ':1883 ' >/dev/null 2>&1
}

rotate_log() {
    FILE=$1
    LIMIT=${2:-1048576}
    [ -f "$FILE" ] || return 0
    SIZE=$(wc -c < "$FILE")
    if [ "$SIZE" -ge "$LIMIT" ]; then
        rm -f "$FILE.1"
        mv "$FILE" "$FILE.1"
    fi
}

mkdir -p "$STATE_DIR"
touch "$LOG_FILE" "$STATE_DIR/openmiio.log"
chmod 600 "$LOG_FILE" "$STATE_DIR/openmiio.log"

if [ ! -x "$OPENMIIO_BIN" ]; then
    echo "missing executable: $OPENMIIO_BIN" >&2
    exit 1
fi
if [ ! -x "$BRIDGE_BIN" ]; then
    echo "missing executable: $BRIDGE_BIN" >&2
    exit 1
fi

if ! openmiio_is_running; then
	rotate_log "$STATE_DIR/openmiio.log"
	touch "$STATE_DIR/openmiio.log"
	chmod 600 "$STATE_DIR/openmiio.log"
    "$OPENMIIO_BIN" miio central mqtt cache >>"$STATE_DIR/openmiio.log" 2>&1 &
    WAIT=0
    while [ "$WAIT" -lt 20 ] && { ! openmiio_is_running || ! mqtt_is_ready; }; do
        sleep 1
        WAIT=$((WAIT + 1))
    done
    if ! openmiio_is_running || ! mqtt_is_ready; then
        echo "openmiio_agent failed to start; log: $STATE_DIR/openmiio.log" >&2
        exit 1
    fi
else
    echo "openmiio_agent is already running with miio central mqtt cache"
fi

if [ -f "$PID_FILE" ]; then
    OLD_PID=$(cat "$PID_FILE")
    if bridge_pid_is_running "$OLD_PID"; then
        echo "mgl03-homekit-bridge is already running (pid $OLD_PID)"
        exit 0
    fi
    rm -f "$PID_FILE"
fi

rotate_log "$LOG_FILE"
touch "$LOG_FILE"
chmod 600 "$LOG_FILE"

GOMAXPROCS="${GOMAXPROCS:-1}" \
GOMEMLIMIT="${GOMEMLIMIT:-16MiB}" \
GOGC="${GOGC:-20}" \
    "$BRIDGE_BIN" -config "$CONFIG_FILE" >>"$LOG_FILE" 2>&1 &
BRIDGE_PID=$!
echo "$BRIDGE_PID" >"$PID_FILE"

sleep 2
if ! bridge_pid_is_running "$BRIDGE_PID"; then
    wait "$BRIDGE_PID"
    EXIT_CODE=$?
    rm -f "$PID_FILE"
    echo "mgl03-homekit-bridge exited during startup (status $EXIT_CODE)" >&2
    tail -20 "$LOG_FILE" >&2
    exit "$EXIT_CODE"
fi

echo "started mgl03-homekit-bridge (pid $BRIDGE_PID); log: $LOG_FILE"
