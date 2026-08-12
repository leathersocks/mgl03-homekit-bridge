#!/bin/sh
set -u

STATE_DIR=/data/mgl03-homekit
STAGING_DIR="$STATE_DIR/install-staging"
MANIFEST_FILE="$STAGING_DIR/manifest.txt"
CHANGES_FILE="$STAGING_DIR/changes.txt"
BACKUP_SUFFIX=.pre-notelnet-install
KNOWN_STARTUP_MD5=b8086d3d3f450bc66156ed5ef9a38520

fail() {
    echo "INSTALL ERROR: $1" >&2
    exit "$2"
}

file_md5() {
    md5sum "$1" | cut -d ' ' -f 1
}

file_sha256() {
    sha256sum "$1" 2>/dev/null | cut -d ' ' -f 1
}

verify_file() {
    FILE=$1
    EXPECTED_SHA256=$2
    EXPECTED_MD5=$3
    ACTUAL_SHA256=$(file_sha256 "$FILE")
    if [ -n "$ACTUAL_SHA256" ]; then
        [ "$ACTUAL_SHA256" = "$EXPECTED_SHA256" ]
        return $?
    fi
    [ "$(file_md5 "$FILE")" = "$EXPECTED_MD5" ]
}

allowed_destination() {
    case "$1" in
        /data/openmiio_agent|\
        /data/mgl03-homekit-bridge|\
        /data/mgl03-homekit-start.sh|\
        /data/mgl03-homekit-stop.sh|\
        /data/mgl03-homekit-cleanup.sh|\
        /data/scripts/startup.sh)
            return 0
            ;;
    esac
    return 1
}

rollback_changes() {
    if [ ! -f "$CHANGES_FILE" ]; then
        return 0
    fi

    echo "rolling back changed runtime files" >&2
    while read -r DESTINATION HAD_BACKUP; do
        [ -n "$DESTINATION" ] || continue
        rm -f "$DESTINATION"
        if [ "$HAD_BACKUP" = "1" ]; then
            mv "$DESTINATION$BACKUP_SUFFIX" "$DESTINATION"
        fi
    done < "$CHANGES_FILE"
    sync
}

discard_backups() {
    if [ ! -f "$CHANGES_FILE" ]; then
        return 0
    fi

    while read -r DESTINATION HAD_BACKUP; do
        [ -n "$DESTINATION" ] || continue
        if [ "$HAD_BACKUP" = "1" ]; then
            rm -f "$DESTINATION$BACKUP_SUFFIX"
        fi
    done < "$CHANGES_FILE"
}

if [ "$#" -ne 3 ]; then
    fail "usage: install-on-device.sh BASE_URL MANIFEST_SHA256 MANIFEST_MD5" 2
fi

BASE_URL=$1
EXPECTED_MANIFEST_SHA256=$2
EXPECTED_MANIFEST_MD5=$3

case "$BASE_URL" in
    http://*) ;;
    *) fail "BASE_URL must use HTTP on the trusted local network" 3 ;;
esac

if [ "${#EXPECTED_MANIFEST_SHA256}" -ne 64 ]; then
    fail "MANIFEST_SHA256 must be 64 lowercase hexadecimal characters" 4
fi
case "$EXPECTED_MANIFEST_SHA256$EXPECTED_MANIFEST_MD5" in
    *[!0-9a-f]*) fail "manifest checksums must be lowercase hexadecimal" 4 ;;
esac
if [ "${#EXPECTED_MANIFEST_MD5}" -ne 32 ]; then
    fail "MANIFEST_MD5 must be 32 lowercase hexadecimal characters" 4
fi

rm -rf "$STAGING_DIR"
mkdir -p "$STAGING_DIR" /data/scripts || fail "create staging directories" 5

wget -O "$MANIFEST_FILE.new" "$BASE_URL/manifest.txt" || \
    fail "download manifest" 10

if ! verify_file "$MANIFEST_FILE.new" "$EXPECTED_MANIFEST_SHA256" "$EXPECTED_MANIFEST_MD5"; then
    fail "manifest checksum mismatch" 11
fi
mv "$MANIFEST_FILE.new" "$MANIFEST_FILE" || fail "activate manifest" 12

echo "downloading and verifying installation artifacts"
while read -r EXPECTED_SHA256 EXPECTED_MD5 MODE NAME DESTINATION EXTRA; do
    [ -n "$EXPECTED_SHA256" ] || continue
    [ -z "$EXTRA" ] || fail "malformed manifest entry for $NAME" 13

    allowed_destination "$DESTINATION" || \
        fail "manifest contains unsafe destination: $DESTINATION" 14

    case "$MODE" in
        700|755) ;;
        *) fail "manifest contains unsafe mode for $NAME" 15 ;;
    esac

    case "$NAME" in
        ''|*[!A-Za-z0-9._-]*) fail "manifest contains unsafe filename" 16 ;;
    esac

    STAGED_FILE="$STAGING_DIR/$NAME"
    wget -O "$STAGED_FILE" "$BASE_URL/$NAME" || \
        fail "download artifact: $NAME" 17

    if ! verify_file "$STAGED_FILE" "$EXPECTED_SHA256" "$EXPECTED_MD5"; then
        fail "artifact checksum mismatch: $NAME" 18
    fi
done < "$MANIFEST_FILE"

NEW_STARTUP_MD5=
while read -r EXPECTED_SHA256 EXPECTED_MD5 MODE NAME DESTINATION EXTRA; do
    if [ "$DESTINATION" = "/data/scripts/startup.sh" ]; then
        NEW_STARTUP_MD5=$EXPECTED_MD5
        break
    fi
done < "$MANIFEST_FILE"
[ -n "$NEW_STARTUP_MD5" ] || fail "manifest is missing startup.sh" 19

if [ -e /data/scripts/startup.sh ]; then
    CURRENT_STARTUP_MD5=$(file_md5 /data/scripts/startup.sh)
    case "$CURRENT_STARTUP_MD5" in
        "$NEW_STARTUP_MD5"|"$KNOWN_STARTUP_MD5") ;;
        *)
            fail "refusing to overwrite an unknown /data/scripts/startup.sh" 20
            ;;
    esac
fi

if [ -x /data/mgl03-homekit-stop.sh ]; then
    /data/mgl03-homekit-stop.sh || fail "stop current bridge" 21
fi

: > "$CHANGES_FILE"
INSTALL_FAILED=0
while read -r EXPECTED_SHA256 EXPECTED_MD5 MODE NAME DESTINATION EXTRA; do
    [ -n "$EXPECTED_SHA256" ] || continue
    STAGED_FILE="$STAGING_DIR/$NAME"

    if [ -e "$DESTINATION" ]; then
        CURRENT_MD5=$(file_md5 "$DESTINATION")
        if [ "$CURRENT_MD5" = "$EXPECTED_MD5" ]; then
            chmod "$MODE" "$DESTINATION" || INSTALL_FAILED=1
            rm -f "$STAGED_FILE"
            [ "$INSTALL_FAILED" -eq 0 ] || break
            continue
        fi
    fi

    if [ -e "$DESTINATION$BACKUP_SUFFIX" ]; then
        echo "stale installer backup blocks update: $DESTINATION$BACKUP_SUFFIX" >&2
        INSTALL_FAILED=1
        break
    fi

    HAD_BACKUP=0
    if [ -e "$DESTINATION" ]; then
        mv "$DESTINATION" "$DESTINATION$BACKUP_SUFFIX" || INSTALL_FAILED=1
        HAD_BACKUP=1
    fi
    [ "$INSTALL_FAILED" -eq 0 ] || break

    echo "$DESTINATION $HAD_BACKUP" >> "$CHANGES_FILE"
    mv "$STAGED_FILE" "$DESTINATION" || INSTALL_FAILED=1
    chmod "$MODE" "$DESTINATION" || INSTALL_FAILED=1
    [ "$INSTALL_FAILED" -eq 0 ] || break
done < "$MANIFEST_FILE"

if [ "$INSTALL_FAILED" -ne 0 ]; then
    rollback_changes
    if [ -x /data/mgl03-homekit-start.sh ]; then
        /data/mgl03-homekit-start.sh >/dev/null 2>&1 || true
    fi
    fail "install runtime files; previous files were restored" 30
fi

sync
if ! /data/mgl03-homekit-start.sh; then
    rollback_changes
    /data/mgl03-homekit-start.sh >/dev/null 2>&1 || true
    fail "start new bridge; previous files were restored" 40
fi

sleep 3
BRIDGE_PID=
if [ -f /data/mgl03-homekit/bridge.pid ]; then
    BRIDGE_PID=$(cat /data/mgl03-homekit/bridge.pid)
fi
if [ -z "$BRIDGE_PID" ] || ! kill -0 "$BRIDGE_PID" 2>/dev/null; then
    /data/mgl03-homekit-stop.sh >/dev/null 2>&1 || true
    rollback_changes
    /data/mgl03-homekit-start.sh >/dev/null 2>&1 || true
    fail "bridge did not survive startup; previous files were restored" 41
fi

discard_backups
rm -rf "$STAGING_DIR"
sync

echo "installation complete; mgl03-homekit-bridge pid=$BRIDGE_PID"
exit 0
