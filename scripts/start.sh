#!/bin/sh
set -u

STATE_DIR=/data/mgl03-homekit
BRIDGE_BIN=/data/mgl03-homekit-bridge
OPENMIIO_BIN=/data/openmiio_agent
CONFIG_FILE="$STATE_DIR/config.json"
LOG_FILE="$STATE_DIR/bridge.log"
PID_FILE="$STATE_DIR/bridge.pid"

mkdir -p "$STATE_DIR"

if [ ! -x "$OPENMIIO_BIN" ]; then
    echo "missing executable: $OPENMIIO_BIN" >&2
    exit 1
fi
if [ ! -x "$BRIDGE_BIN" ]; then
    echo "missing executable: $BRIDGE_BIN" >&2
    exit 1
fi

if ! pidof openmiio_agent >/dev/null 2>&1; then
    "$OPENMIIO_BIN" miio central mqtt cache >>"$STATE_DIR/openmiio.log" 2>&1 &
    sleep 2
else
    echo "openmiio_agent is already running; make sure it was started with central mqtt cache"
fi

if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    echo "mgl03-homekit-bridge is already running"
    exit 0
fi

"$BRIDGE_BIN" -config "$CONFIG_FILE" >>"$LOG_FILE" 2>&1 &
echo $! >"$PID_FILE"
echo "started mgl03-homekit-bridge (pid $(cat "$PID_FILE")); log: $LOG_FILE"
