#!/usr/bin/env python3
"""Temporarily enable Telnet on an MGL03 running firmware before 1.5.5."""

from __future__ import annotations

import argparse
import getpass
import os
import re
import socket
import sys
import time

from miio import Device


TELNET_COMMAND = (
    "passwd -d $USER; "
    "riu_w 101e 53 3012 || echo enable > /sys/class/tty/tty/enable; "
    "telnetd"
)


def port_is_open(host: str, port: int, timeout: float = 1.0) -> bool:
    try:
        with socket.create_connection((host, port), timeout=timeout):
            return True
    except OSError:
        return False


def main() -> int:
    parser = argparse.ArgumentParser(
        description=(
            "Enable temporary Telnet on lumi.gateway.mgl03 firmware 1.5.0-1.5.4 "
            "using the local miIO token. The token is prompted without echo."
        )
    )
    parser.add_argument("--gateway-ip", default="192.168.10.41")
    parser.add_argument("--telnet-port", type=int, default=23)
    args = parser.parse_args()

    if port_is_open(args.gateway_ip, args.telnet_port):
        print(f"Telnet is already open at {args.gateway_ip}:{args.telnet_port}.")
        return 0

    token = os.environ.get("MGL03_TOKEN") or getpass.getpass(
        "MGL03 miIO token (32 hex characters; input is hidden): "
    )
    token = token.strip()
    if not re.fullmatch(r"[0-9a-fA-F]{32}", token):
        print("ERROR: token must be exactly 32 hexadecimal characters.", file=sys.stderr)
        return 2

    print(f"Sending the firmware 1.5.0-1.5.4 Telnet-enable request to {args.gateway_ip}...")
    device = Device(ip=args.gateway_ip, token=token, timeout=5)
    try:
        result = device.send(
            "set_ip_info",
            {"ssid": '""', "pswd": "1; " + TELNET_COMMAND},
            retry_count=1,
        )
    except Exception as exc:
        print(f"ERROR: miIO request failed: {exc}", file=sys.stderr)
        return 3
    finally:
        token = ""

    if result != ["ok"]:
        print(f"ERROR: unexpected gateway response: {result!r}", file=sys.stderr)
        return 4

    for _ in range(15):
        if port_is_open(args.gateway_ip, args.telnet_port):
            print(f"Telnet is open at {args.gateway_ip}:{args.telnet_port}.")
            return 0
        time.sleep(1)

    print("ERROR: gateway returned ok, but the Telnet port did not open.", file=sys.stderr)
    return 5


if __name__ == "__main__":
    raise SystemExit(main())
