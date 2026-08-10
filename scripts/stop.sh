#!/bin/sh
set -u

PID_FILE=/data/mgl03-homekit/bridge.pid

if [ ! -f "$PID_FILE" ]; then
    echo "mgl03-homekit-bridge is not running"
    exit 0
fi

PID=$(cat "$PID_FILE")
if kill -0 "$PID" 2>/dev/null; then
    kill "$PID"
    echo "stopped mgl03-homekit-bridge (pid $PID)"
else
    echo "removed stale pid file"
fi
rm -f "$PID_FILE"
