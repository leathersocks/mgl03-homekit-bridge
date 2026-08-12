#!/usr/bin/env python3
"""Install or update mgl03-homekit-bridge through local miIO, without Telnet."""

from __future__ import annotations

import argparse
import getpass
import hashlib
import http.server
import ipaddress
from pathlib import Path
import re
import secrets
import shlex
import socket
import sys
import tempfile
import threading
import time
from urllib.parse import unquote, urlparse
import urllib.request

try:
    from miio import Device
except ImportError:  # pragma: no cover - exercised only on an unprepared PC
    Device = None


OPENMIIO_URL = (
    "https://github.com/AlexxIT/openmiio_agent/releases/download/"
    "v1.2.1/openmiio_agent_mips"
)
OPENMIIO_MD5 = "6c3f4dca62647b9d19a81e1ccaa5ccc0"
OPENMIIO_SHA256 = "78c775b354bb5fb896682fd3c26b9114cf336387985629ca16bc40a19cfb74f6"
MODEL = "lumi.gateway.mgl03"

PROJECT_ROOT = Path(__file__).resolve().parents[1]

ARTIFACTS = (
    ("openmiio_agent", "/data/openmiio_agent", "755"),
    ("mgl03-homekit-bridge", "/data/mgl03-homekit-bridge", "755"),
    ("mgl03-homekit-start.sh", "/data/mgl03-homekit-start.sh", "755"),
    ("mgl03-homekit-stop.sh", "/data/mgl03-homekit-stop.sh", "755"),
    ("mgl03-homekit-cleanup.sh", "/data/mgl03-homekit-cleanup.sh", "755"),
    ("startup.sh", "/data/scripts/startup.sh", "755"),
)

STATUS_MESSAGES = {
    0: "installation completed",
    10: "gateway could not download the manifest",
    11: "manifest checksum mismatch",
    17: "gateway could not download an artifact",
    18: "artifact checksum mismatch",
    20: "an unknown existing startup hook was preserved",
    30: "runtime file installation failed and was rolled back",
    40: "new bridge start failed and was rolled back",
    41: "new bridge exited during startup and was rolled back",
    90: "gateway could not download the on-device installer",
    91: "on-device installer checksum mismatch",
}


def md5_bytes(data: bytes) -> str:
    return hashlib.md5(data).hexdigest()


def md5_file(path: Path) -> str:
    digest = hashlib.md5()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def supported_firmware(version: str | None) -> bool:
    if not version:
        return False
    return re.match(r"^1\.5\.[0-4](?:_|$)", version) is not None


def ipv4_address(value: str) -> str:
    try:
        return str(ipaddress.IPv4Address(value))
    except ipaddress.AddressValueError as exc:
        raise argparse.ArgumentTypeError(f"invalid IPv4 address: {value}") from exc


def validate_mips_binary(path: Path) -> None:
    header = path.read_bytes()[:20]
    if len(header) < 20 or header[:4] != b"\x7fELF":
        raise ValueError(f"not an ELF executable: {path}")
    if header[5] != 1:
        raise ValueError(f"binary is not little-endian: {path}")
    machine = int.from_bytes(header[18:20], "little")
    if machine != 8:
        raise ValueError(f"binary is not MIPS (e_machine={machine}): {path}")


def discover_source_ip(gateway_ip: str) -> str:
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
        sock.connect((gateway_ip, 54321))
        return sock.getsockname()[0]


def read_lf_script(path: Path) -> bytes:
    data = path.read_bytes()
    if b"\r\n" in data:
        raise ValueError(f"script must use LF line endings: {path}")
    return data


def obtain_openmiio(path: Path | None) -> bytes:
    if path is not None:
        data = path.read_bytes()
    else:
        cached = PROJECT_ROOT / "bin" / "openmiio_agent"
        if cached.is_file():
            data = cached.read_bytes()
        else:
            print("Downloading official openmiio_agent v1.2.1 for MIPS...")
            request = urllib.request.Request(
                OPENMIIO_URL,
                headers={"User-Agent": "mgl03-homekit-no-telnet-installer/1.0"},
            )
            with urllib.request.urlopen(request, timeout=60) as response:
                data = response.read()

    actual_md5 = md5_bytes(data)
    actual_sha256 = sha256_bytes(data)
    if actual_md5 != OPENMIIO_MD5 or actual_sha256 != OPENMIIO_SHA256:
        raise ValueError(
            "openmiio_agent checksum mismatch: "
            f"sha256={actual_sha256}, md5={actual_md5}"
        )
    return data


def prepare_stage(stage: Path, openmiio_data: bytes, bridge_path: Path) -> tuple[str, str]:
    validate_mips_binary(bridge_path)

    sources: dict[str, bytes] = {
        "openmiio_agent": openmiio_data,
        "mgl03-homekit-bridge": bridge_path.read_bytes(),
        "mgl03-homekit-start.sh": read_lf_script(PROJECT_ROOT / "scripts" / "start.sh"),
        "mgl03-homekit-stop.sh": read_lf_script(PROJECT_ROOT / "scripts" / "stop.sh"),
        "mgl03-homekit-cleanup.sh": read_lf_script(PROJECT_ROOT / "scripts" / "cleanup.sh"),
        "startup.sh": read_lf_script(PROJECT_ROOT / "scripts" / "startup.sh"),
    }

    manifest_lines: list[str] = []
    for name, destination, mode in ARTIFACTS:
        data = sources[name]
        (stage / name).write_bytes(data)
        manifest_lines.append(
            f"{sha256_bytes(data)} {md5_bytes(data)} {mode} {name} {destination}"
        )

    manifest = ("\n".join(manifest_lines) + "\n").encode()
    (stage / "manifest.txt").write_bytes(manifest)

    installer = read_lf_script(PROJECT_ROOT / "scripts" / "install-on-device.sh")
    (stage / "install-on-device.sh").write_bytes(installer)
    return sha256_bytes(manifest), md5_bytes(manifest)


def build_bootstrap(
    base_url: str,
    callback_url: str,
    manifest_sha256: str,
    manifest_md5: str,
    installer_sha256: str,
    installer_md5: str,
) -> bytes:
    values = {
        "BASE_URL": base_url,
        "CALLBACK_URL": callback_url,
        "MANIFEST_SHA256": manifest_sha256,
        "MANIFEST_MD5": manifest_md5,
        "INSTALLER_SHA256": installer_sha256,
        "INSTALLER_MD5": installer_md5,
    }
    assignments = "\n".join(f"{key}={shlex.quote(value)}" for key, value in values.items())
    script = f"""#!/bin/sh
set -u
{assignments}
STATE_DIR=/data/mgl03-homekit
INSTALLER=$STATE_DIR/install-on-device.sh
STATUS=90

verify_file() {{
    FILE=$1
    EXPECTED_SHA256=$2
    EXPECTED_MD5=$3
    ACTUAL_SHA256=$(sha256sum "$FILE" 2>/dev/null | cut -d ' ' -f 1)
    if [ -n "$ACTUAL_SHA256" ]; then
        [ "$ACTUAL_SHA256" = "$EXPECTED_SHA256" ]
        return $?
    fi
    [ "$(md5sum "$FILE" | cut -d ' ' -f 1)" = "$EXPECTED_MD5" ]
}}

mkdir -p "$STATE_DIR"
if wget -O "$INSTALLER.new" "$BASE_URL/install-on-device.sh"; then
    if verify_file "$INSTALLER.new" "$INSTALLER_SHA256" "$INSTALLER_MD5"; then
        chmod 700 "$INSTALLER.new"
        mv "$INSTALLER.new" "$INSTALLER"
        "$INSTALLER" "$BASE_URL" "$MANIFEST_SHA256" "$MANIFEST_MD5"
        STATUS=$?
    else
        STATUS=91
    fi
fi

wget -O /dev/null "$CALLBACK_URL/$STATUS" >/dev/null 2>&1 || true
exit "$STATUS"
"""
    return script.encode()


def build_injection(base_url: str, bootstrap_sha256: str, bootstrap_md5: str) -> str:
    state_dir = "/data/mgl03-homekit"
    remote = f"{state_dir}/no-telnet-bootstrap.sh"
    temporary = remote + ".new"
    bootstrap_url = f"{base_url}/bootstrap.sh"
    command = (
        f"mkdir -p {state_dir}; "
        f"(wget -O {temporary} {bootstrap_url} && "
        f'(S=$(sha256sum {temporary} 2>/dev/null | cut -d \' \' -f 1); '
        f'if [ -n "$S" ]; then [ "$S" = "{bootstrap_sha256}" ]; '
        f'else [ "$(md5sum {temporary} | cut -d \' \' -f 1)" = "{bootstrap_md5}" ]; fi) && '
        f"chmod 700 {temporary} && mv {temporary} {remote} && {remote}) "
        f">{state_dir}/install.log 2>&1 &"
    )
    if len(command.encode()) > 700:
        raise ValueError("bootstrap injection command is unexpectedly large")
    return command


def make_handler(stage: Path, nonce: str, event: threading.Event, result: dict[str, int]):
    class Handler(http.server.SimpleHTTPRequestHandler):
        def __init__(self, *args, **kwargs):
            super().__init__(*args, directory=str(stage), **kwargs)

        def do_GET(self):  # noqa: N802 - inherited HTTP method name
            path = unquote(urlparse(self.path).path)
            prefix = f"/callback/{nonce}/"
            if path.startswith(prefix):
                try:
                    status = int(path[len(prefix) :])
                except ValueError:
                    self.send_error(400)
                    return
                result["status"] = status
                self.send_response(200)
                self.send_header("Content-Type", "text/plain")
                self.end_headers()
                self.wfile.write(b"ok\n")
                event.set()
                return
            super().do_GET()

        def log_message(self, fmt, *args):
            if "/callback/" in str(args[0] if args else ""):
                return
            print("HTTP:", fmt % args)

    return Handler


def port_is_open(host: str, port: int, timeout: float = 1.0) -> bool:
    try:
        with socket.create_connection((host, port), timeout=timeout):
            return True
    except OSError:
        return False


def run(args: argparse.Namespace) -> int:
    if Device is None:
        print(
            "ERROR: python-miio is required. Install it with: py -m pip install python-miio",
            file=sys.stderr,
        )
        return 2

    token = getpass.getpass(
        "MGL03 miIO token (32 hex characters; input is hidden): "
    )
    token = token.strip()
    if not re.fullmatch(r"[0-9a-fA-F]{32}", token):
        print("ERROR: token must be exactly 32 hexadecimal characters.", file=sys.stderr)
        return 2

    print(f"Checking {args.gateway_ip} through local miIO...")
    device = Device(ip=args.gateway_ip, token=token, timeout=5)
    try:
        info = device.info()
    except Exception as exc:
        print(f"ERROR: miIO device check failed: {exc}", file=sys.stderr)
        return 3

    if info.model != MODEL:
        print(f"ERROR: expected {MODEL}, got {info.model!r}.", file=sys.stderr)
        return 4
    if not supported_firmware(info.firmware_version):
        print(
            "ERROR: no-Telnet set_ip_info installation is limited to MGL03 firmware "
            f"1.5.0-1.5.4; detected {info.firmware_version!r}.",
            file=sys.stderr,
        )
        return 5
    print(f"Gateway verified: {info.model}, firmware {info.firmware_version}.")

    openmiio_path = Path(args.openmiio_bin).resolve() if args.openmiio_bin else None
    try:
        openmiio_data = obtain_openmiio(openmiio_path)
    except Exception as exc:
        print(f"ERROR: prepare openmiio_agent: {exc}", file=sys.stderr)
        return 6

    bridge_path = Path(args.bridge_bin).resolve()
    callback_event = threading.Event()
    callback_result: dict[str, int] = {}
    nonce = secrets.token_hex(12)

    with tempfile.TemporaryDirectory(prefix="mgl03-homekit-install-") as temp:
        stage = Path(temp)
        try:
            manifest_sha256, manifest_md5 = prepare_stage(stage, openmiio_data, bridge_path)
        except Exception as exc:
            print(f"ERROR: prepare installation files: {exc}", file=sys.stderr)
            return 7

        pc_ip = args.pc_ip or discover_source_ip(args.gateway_ip)
        pc_ip = ipv4_address(pc_ip)
        handler = make_handler(stage, nonce, callback_event, callback_result)
        try:
            server = http.server.ThreadingHTTPServer((args.bind, args.http_port), handler)
        except OSError as exc:
            print(f"ERROR: start temporary HTTP server: {exc}", file=sys.stderr)
            return 8
        server.daemon_threads = True
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()

        base_url = f"http://{pc_ip}:{server.server_port}"
        callback_url = f"{base_url}/callback/{nonce}"
        installer_sha256 = sha256_file(stage / "install-on-device.sh")
        installer_md5 = md5_file(stage / "install-on-device.sh")
        bootstrap = build_bootstrap(
            base_url,
            callback_url,
            manifest_sha256,
            manifest_md5,
            installer_sha256,
            installer_md5,
        )
        (stage / "bootstrap.sh").write_bytes(bootstrap)
        bootstrap_sha256 = sha256_bytes(bootstrap)
        bootstrap_md5 = md5_bytes(bootstrap)
        injection = build_injection(base_url, bootstrap_sha256, bootstrap_md5)

        print(f"Serving a credential-free installation bundle at {base_url}.")
        print("Requesting the gateway to install it; no Telnet session will be opened...")
        try:
            response = device.send(
                "set_ip_info",
                {"ssid": '""', "pswd": "1; " + injection},
                retry_count=1,
            )
        except Exception as exc:
            server.shutdown()
            print(f"ERROR: send miIO installation request: {exc}", file=sys.stderr)
            return 9
        finally:
            token = ""

        if response != ["ok"]:
            server.shutdown()
            print(f"ERROR: unexpected gateway response: {response!r}", file=sys.stderr)
            return 10

        print(f"Gateway accepted the request. Waiting up to {args.timeout} seconds...")
        completed = callback_event.wait(args.timeout)
        server.shutdown()
        thread.join(timeout=5)
        if not completed:
            print(
                "ERROR: installer callback timed out. The gateway log is "
                "/data/mgl03-homekit/install.log.",
                file=sys.stderr,
            )
            return 11

        status = callback_result.get("status", 99)
        if status != 0:
            message = STATUS_MESSAGES.get(status, "unrecognized gateway installer failure")
            print(f"ERROR: gateway installer status {status}: {message}.", file=sys.stderr)
            return 12

    print("Gateway reported a successful atomic installation.")
    # New installations may spend 30 seconds collecting all supported sensors
    # before the HAP listener starts, so leave an additional readiness margin.
    for _ in range(60):
        if port_is_open(args.gateway_ip, 51826):
            print("HomeKit bridge is listening on TCP 51826.")
            return 0
        time.sleep(1)

    print(
        "Installation succeeded. TCP 51826 is not open yet; on a fresh install the "
        "bridge may still be waiting for its first supported BLE advertisement."
    )
    return 0


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Install or update mgl03-homekit-bridge through the local miIO "
            "set_ip_info path without opening a Telnet session."
        )
    )
    parser.add_argument("--gateway-ip", type=ipv4_address, default="192.168.10.41")
    parser.add_argument(
        "--pc-ip",
        type=ipv4_address,
        help="PC address reachable by the gateway; auto-detected",
    )
    parser.add_argument(
        "--bind",
        type=ipv4_address,
        default="0.0.0.0",
        help="temporary HTTP bind address",
    )
    parser.add_argument("--http-port", type=int, default=0, help="temporary HTTP port; 0 selects one")
    parser.add_argument("--timeout", type=int, default=180)
    parser.add_argument(
        "--bridge-bin",
        default=str(PROJECT_ROOT / "bin" / "mgl03-homekit-bridge"),
    )
    parser.add_argument(
        "--openmiio-bin",
        help="optional pre-downloaded official openmiio_agent_mips binary",
    )
    return parser.parse_args(argv)


def main() -> int:
    return run(parse_args())


if __name__ == "__main__":
    raise SystemExit(main())
