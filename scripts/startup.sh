#!/bin/sh

# Firmware 1.5.0_0026 calls this file from /etc/init.d/rcS. Schedule the
# HomeKit bridge before handing control to the stock startup process, which
# remains resident and does not return during normal operation.
MGL03_STOCK_START_SCRIPT=/bin/startup.sh
MGL03_HOMEKIT_STATE_DIR=/data/mgl03-homekit
MGL03_HOMEKIT_START_SCRIPT=/data/mgl03-homekit-start.sh
MGL03_HOMEKIT_STARTUP_LOG="$MGL03_HOMEKIT_STATE_DIR/startup.log"

mkdir -p "$MGL03_HOMEKIT_STATE_DIR"
if [ -f "$MGL03_HOMEKIT_STARTUP_LOG" ] && [ "$(wc -c < "$MGL03_HOMEKIT_STARTUP_LOG")" -ge 262144 ]; then
    rm -f "$MGL03_HOMEKIT_STARTUP_LOG.1"
    mv "$MGL03_HOMEKIT_STARTUP_LOG" "$MGL03_HOMEKIT_STARTUP_LOG.1"
fi

# rcS runs this custom hook instead of the stock startup command.
if [ ! -x "$MGL03_STOCK_START_SCRIPT" ]; then
    echo "missing executable: $MGL03_STOCK_START_SCRIPT" >&2
    exit 1
fi
(
	sleep "${MGL03_HOMEKIT_BOOT_DELAY:-5}"
	WAIT=0
	while [ "$WAIT" -lt "${MGL03_HOMEKIT_READY_TIMEOUT:-60}" ]; do
		if netstat -lnt 2>/dev/null | grep -e ':1883 ' >/dev/null 2>&1; then
			break
		fi
		sleep 1
		WAIT=$((WAIT + 1))
	done

	echo "$(date) starting mgl03-homekit-bridge after boot"
    if [ ! -x "$MGL03_HOMEKIT_START_SCRIPT" ]; then
        echo "missing executable: $MGL03_HOMEKIT_START_SCRIPT" >&2
        exit 1
    fi

    "$MGL03_HOMEKIT_START_SCRIPT"
) >>"$MGL03_HOMEKIT_STARTUP_LOG" 2>&1 &

# Preserve the firmware's normal process lifetime and exit status.
exec "$MGL03_STOCK_START_SCRIPT"
