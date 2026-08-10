#!/bin/sh
set -u

STATE_DIR=/data/mgl03-homekit
BRIDGE_BIN=/data/mgl03-homekit-bridge
OPENMIIO_BIN=/data/openmiio_agent
CONFIG_FILE="$STATE_DIR/config.json"
LOG_FILE="$STATE_DIR/bridge.log"
PID_FILE="$STATE_DIR/bridge.pid"

openmiio_is_running() {
    ps | grep '[o]penmiio_agent' >/dev/null 2>&1
}

mkdir -p "$STATE_DIR"

if [ ! -x "$OPENMIIO_BIN" ]; then
    echo "missing executable: $OPENMIIO_BIN" >&2
    exit 1
fi
if [ ! -x "$BRIDGE_BIN" ]; then
    echo "missing executable: $BRIDGE_BIN" >&2
    exit 1
fi

if ! openmiio_is_running; then
    "$OPENMIIO_BIN" miio central mqtt cache >>"$STATE_DIR/openmiio.log" 2>&1 &
    sleep 2
    if ! openmiio_is_running; then
        echo "openmiio_agent failed to start; log: $STATE_DIR/openmiio.log" >&2
        exit 1
    fi
else
    echo "openmiio_agent is already running; make sure it was started with central mqtt cache"
fi

if [ -f "$PID_FILE" ]; then
    OLD_PID=$(cat "$PID_FILE")
    if kill -0 "$OLD_PID" 2>/dev/null; then
        echo "mgl03-homekit-bridge is already running (pid $OLD_PID)"
        exit 0
    fi
    rm -f "$PID_FILE"
fi

GOMAXPROCS="${GOMAXPROCS:-1}" \
GOMEMLIMIT="${GOMEMLIMIT:-16MiB}" \
GOGC="${GOGC:-20}" \
    "$BRIDGE_BIN" -config "$CONFIG_FILE" >>"$LOG_FILE" 2>&1 &
BRIDGE_PID=$!
echo "$BRIDGE_PID" >"$PID_FILE"

sleep 2
if ! kill -0 "$BRIDGE_PID" 2>/dev/null; then
    wait "$BRIDGE_PID"
    EXIT_CODE=$?
    rm -f "$PID_FILE"
    echo "mgl03-homekit-bridge exited during startup (status $EXIT_CODE)" >&2
    tail -20 "$LOG_FILE" >&2
    exit "$EXIT_CODE"
fi

echo "started mgl03-homekit-bridge (pid $BRIDGE_PID); log: $LOG_FILE"
