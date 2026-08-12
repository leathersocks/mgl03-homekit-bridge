#!/bin/sh
set -u
umask 077

STATE_DIR=/data/mgl03-homekit
OPENMIIO_BIN=/data/openmiio_agent
LOG_FILE="$STATE_DIR/openmiio.log"

openmiio_is_running() {
    for CMDLINE_FILE in /proc/[0-9]*/cmdline; do
        [ -r "$CMDLINE_FILE" ] || continue
        CMDLINE=$(tr '\000' ' ' < "$CMDLINE_FILE")
        case "$CMDLINE" in
            "$OPENMIIO_BIN "*miio*central*mqtt*cache*|\
            "$OPENMIIO_BIN "*miio*mqtt*cache*central*) return 0 ;;
        esac
    done
    return 1
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
if [ ! -x "$OPENMIIO_BIN" ]; then
    echo "missing executable: $OPENMIIO_BIN" >&2
    exit 1
fi

if openmiio_is_running && mqtt_is_ready; then
    echo "openmiio_agent is already running with miio central mqtt cache"
    exit 0
fi

if openmiio_is_running; then
    echo "openmiio_agent is running but MQTT is not ready" >&2
    exit 1
fi

rotate_log "$LOG_FILE"
touch "$LOG_FILE"
chmod 600 "$LOG_FILE"
"$OPENMIIO_BIN" miio central mqtt cache >>"$LOG_FILE" 2>&1 &

WAIT=0
while [ "$WAIT" -lt 20 ] && { ! openmiio_is_running || ! mqtt_is_ready; }; do
    sleep 1
    WAIT=$((WAIT + 1))
done

if ! openmiio_is_running || ! mqtt_is_ready; then
    echo "openmiio_agent failed to start; log: $LOG_FILE" >&2
    exit 1
fi

echo "started openmiio_agent; MQTT is listening on TCP 1883"
