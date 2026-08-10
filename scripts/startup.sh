#!/bin/sh

# Firmware 1.5.0_0026 calls this file from /etc/init.d/rcS. Schedule the
# HomeKit bridge before handing control to the stock startup process, which
# remains resident and does not return during normal operation.
MGL03_STOCK_START_SCRIPT=/bin/startup.sh
MGL03_HOMEKIT_STATE_DIR=/data/mgl03-homekit
MGL03_HOMEKIT_START_SCRIPT=/data/mgl03-homekit-start.sh
MGL03_HOMEKIT_STARTUP_LOG="$MGL03_HOMEKIT_STATE_DIR/startup.log"

mkdir -p "$MGL03_HOMEKIT_STATE_DIR"

# rcS runs this custom hook instead of the stock startup command.
if [ ! -x "$MGL03_STOCK_START_SCRIPT" ]; then
    echo "missing executable: $MGL03_STOCK_START_SCRIPT" >&2
    exit 1
fi
(
    sleep "${MGL03_HOMEKIT_BOOT_DELAY:-30}"

    echo "$(date) starting mgl03-homekit-bridge after boot"
    if [ ! -x "$MGL03_HOMEKIT_START_SCRIPT" ]; then
        echo "missing executable: $MGL03_HOMEKIT_START_SCRIPT" >&2
        exit 1
    fi

    "$MGL03_HOMEKIT_START_SCRIPT"
) >>"$MGL03_HOMEKIT_STARTUP_LOG" 2>&1 &

# Preserve the firmware's normal process lifetime and exit status.
exec "$MGL03_STOCK_START_SCRIPT"
