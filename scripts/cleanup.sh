#!/bin/sh
set -u

APPLY=0
INCLUDE_RECOVERY=0

usage() {
    cat <<'EOF'
Usage: /data/mgl03-homekit-cleanup.sh [--apply] [--include-recovery]

Without --apply, only prints the obsolete files that would be removed.
--include-recovery also removes the pre-multisensor devices.json backup.
EOF
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --apply)
            APPLY=1
            ;;
        --include-recovery)
            INCLUDE_RECOVERY=1
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "unknown option: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
    shift
done

require_path() {
    if [ ! -e "$1" ]; then
        echo "required live path is missing: $1" >&2
        return 1
    fi
}

require_executable() {
    if [ ! -x "$1" ]; then
        echo "required executable is missing: $1" >&2
        return 1
    fi
}

verify_live_install() {
    FAILED=0
    for REQUIRED_PATH in \
        /data/mgl03-homekit/config.json \
        /data/mgl03-homekit/devices.json \
        /data/mgl03-homekit/hap
    do
        require_path "$REQUIRED_PATH" || FAILED=1
    done

    for REQUIRED_EXECUTABLE in \
        /data/openmiio_agent \
        /data/mgl03-homekit-bridge \
        /data/mgl03-homekit-start.sh \
        /data/mgl03-homekit-stop.sh \
        /data/scripts/startup.sh
    do
        require_executable "$REQUIRED_EXECUTABLE" || FAILED=1
    done

    if [ "$FAILED" -ne 0 ]; then
        echo "refusing cleanup because the live installation is incomplete" >&2
        return 1
    fi
}

obsolete_targets() {
    cat <<'EOF'
/data/mgl03-homekit-bridge.go1.26.bak
/data/mgl03-homekit-bridge.go1.26.partial-5181440
/data/mgl03-homekit-bridge.central-only.bak
/data/mgl03-homekit-bridge.dualtopic.bak
/data/mgl03-homekit-bridge.new
/data/mgl03-homekit-bridge.dualtopic
/data/mgl03-homekit-bridge.mdns-reuse
/data/mgl03-runtime-probe
/data/mgl03-ble-db-dump
/data/mgl03-homekit-start.sh.new
/data/mgl03-homekit-start.sh.pidof.bak
/data/mgl03-homekit-stop.sh.new
/data/central-report.log
/data/mgl03-homekit/config.shared-pin.bak
/data/mgl03-homekit/bridge.shared-pin.log
/data/mgl03-homekit/hap.shared-pin.bak
EOF
}

recovery_targets() {
    cat <<'EOF'
/data/mgl03-homekit/devices.before-multisensor.json
EOF
}

is_protected_path() {
    case "$1" in
        /data/openmiio_agent|\
        /data/mgl03-homekit-bridge|\
        /data/mgl03-homekit-start.sh|\
        /data/mgl03-homekit-stop.sh|\
        /data/scripts/startup.sh|\
        /data/mgl03-homekit|\
        /data/mgl03-homekit/config.json|\
        /data/mgl03-homekit/devices.json|\
        /data/mgl03-homekit/hap|\
        /data/mgl03-homekit/bridge.log|\
        /data/mgl03-homekit/openmiio.log|\
        /data/mgl03-homekit/startup.log|\
        /data/mgl03-homekit/bridge.pid)
            return 0
            ;;
    esac
    return 1
}

process_target() {
    TARGET=$1

    case "$TARGET" in
        /data/?*) ;;
        *)
            echo "refusing unsafe path outside /data: $TARGET" >&2
            return 1
            ;;
    esac

    if is_protected_path "$TARGET"; then
        echo "refusing protected live path: $TARGET" >&2
        return 1
    fi

    if [ ! -e "$TARGET" ]; then
        return 0
    fi

    SIZE_LINE=$(du -sk "$TARGET" 2>/dev/null)
    SIZE_KB=${SIZE_LINE%%[!0-9]*}
    if [ -z "$SIZE_KB" ]; then
        SIZE_KB="?"
    fi

    if [ "$APPLY" -eq 0 ]; then
        echo "would remove (${SIZE_KB} KiB): $TARGET"
        return 0
    fi

    echo "removing (${SIZE_KB} KiB): $TARGET"
    if [ -d "$TARGET" ] && [ ! -L "$TARGET" ]; then
        rm -rf "$TARGET"
    else
        rm -f "$TARGET"
    fi
}

verify_live_install || exit 1

if [ "$APPLY" -eq 0 ]; then
    echo "DRY RUN: no files will be deleted"
else
    echo "APPLY MODE: deleting only the reviewed obsolete paths"
fi

echo "Filesystem before cleanup:"
df -k /data

for TARGET in $(obsolete_targets); do
    process_target "$TARGET" || exit 1
done

if [ "$INCLUDE_RECOVERY" -eq 1 ]; then
    for TARGET in $(recovery_targets); do
        process_target "$TARGET" || exit 1
    done
else
    echo "preserving recovery backup: /data/mgl03-homekit/devices.before-multisensor.json"
fi

if [ "$APPLY" -eq 1 ]; then
    sync
    echo "Filesystem after cleanup:"
    df -k /data
else
    echo "Run again with --apply after reviewing the list above."
fi
