#!/usr/bin/env python3
"""Inspect Xiaomi BLE events on an openmiio MQTT broker without dependencies."""

from __future__ import annotations

import argparse
import json
import os
import socket
import struct
import time


DEFAULT_TOPICS = ("miio/report", "central/report")
MAX_PACKET_BYTES = 256 * 1024


def encode_varint(value: int) -> bytes:
    out = bytearray()
    while True:
        digit = value % 128
        value //= 128
        if value:
            digit |= 0x80
        out.append(digit)
        if not value:
            return bytes(out)


def encode_string(value: str) -> bytes:
    data = value.encode("utf-8")
    return struct.pack("!H", len(data)) + data


def read_exact(sock: socket.socket, count: int) -> bytes:
    data = bytearray()
    while len(data) < count:
        chunk = sock.recv(count - len(data))
        if not chunk:
            raise ConnectionError("MQTT connection closed")
        data.extend(chunk)
    return bytes(data)


def read_packet(sock: socket.socket) -> tuple[int, bytes]:
    first = read_exact(sock, 1)[0]
    multiplier = 1
    remaining = 0
    while True:
        digit = read_exact(sock, 1)[0]
        remaining += (digit & 0x7F) * multiplier
        if not digit & 0x80:
            break
        multiplier *= 128
        if multiplier > 128**3:
            raise ValueError("malformed MQTT remaining length")
    if remaining > MAX_PACKET_BYTES:
        raise ValueError(f"MQTT packet exceeds {MAX_PACKET_BYTES} bytes")
    return first, read_exact(sock, remaining)


def connect(host: str, port: int, topics: tuple[str, ...]) -> socket.socket:
    client_id = f"mgl03-ble-probe-{os.getpid()}-{int(time.time())}"
    sock = socket.create_connection((host, port), timeout=5)
    sock.settimeout(5)

    header = encode_string("MQTT") + bytes((4, 2)) + struct.pack("!H", 30)
    payload = encode_string(client_id)
    body = header + payload
    sock.sendall(bytes((0x10,)) + encode_varint(len(body)) + body)

    first, response = read_packet(sock)
    if first >> 4 != 2 or len(response) != 2 or response[1] != 0:
        raise RuntimeError(f"CONNACK failed: {response!r}")

    body = struct.pack("!H", 1)
    for topic in topics:
        body += encode_string(topic) + b"\x00"
    sock.sendall(bytes((0x82,)) + encode_varint(len(body)) + body)

    first, response = read_packet(sock)
    if first >> 4 != 9 or len(response) != 2 + len(topics):
        raise RuntimeError(f"malformed SUBACK: {response!r}")
    if response[:2] != b"\x00\x01" or any(code != 0 for code in response[2:]):
        raise RuntimeError(f"SUBACK rejected a topic: {response!r}")
    return sock


def parse_publish(first: int, body: bytes) -> tuple[str, bytes]:
    if len(body) < 2:
        raise ValueError("short MQTT PUBLISH")
    topic_length = struct.unpack("!H", body[:2])[0]
    position = 2 + topic_length
    if len(body) < position:
        raise ValueError("short MQTT topic")
    topic = body[2:position].decode("utf-8", "replace")
    qos = (first >> 1) & 0x03
    if qos == 3:
        raise ValueError("invalid MQTT QoS")
    if qos:
        position += 2
    if len(body) < position:
        raise ValueError("short MQTT packet identifier")
    return topic, body[position:]


def find_ble_events(node):
    if isinstance(node, dict):
        if node.get("method") == "_async.ble_event" and isinstance(node.get("params"), dict):
            yield node["params"]
        for value in node.values():
            yield from find_ble_events(value)
    elif isinstance(node, list):
        for value in node:
            yield from find_ble_events(value)


def print_event(topic: str, event: dict) -> None:
    device = event.get("dev") or {}
    print("\n--- BLE EVENT ---")
    print("topic :", topic)
    print("did   :", device.get("did"))
    print("mac   :", device.get("mac"))
    print("pdid  :", device.get("pdid"))
    print("frmCnt:", event.get("frmCnt"))
    print("gwts  :", event.get("gwts"))
    for item in event.get("evt") or []:
        print(f"evt   : eid={item.get('eid')} edata={item.get('edata')}")


def run(host: str, port: int, topics: tuple[str, ...], reconnect: float) -> None:
    print(f"Target: mqtt://{host}:{port} topics={','.join(topics)}")
    print("Press Ctrl+C to stop.")
    while True:
        sock = None
        try:
            sock = connect(host, port, topics)
            print("Subscribed; waiting for BLE events...")
            last_outbound = time.monotonic()
            ping_sent = None
            while True:
                now = time.monotonic()
                if ping_sent is not None and now - ping_sent >= 10:
                    raise TimeoutError("PINGRESP timeout")
                if ping_sent is None and now - last_outbound >= 15:
                    sock.sendall(b"\xC0\x00")
                    last_outbound = now
                    ping_sent = now
                try:
                    first, body = read_packet(sock)
                except socket.timeout:
                    continue
                packet_type = first >> 4
                if packet_type == 13:
                    ping_sent = None
                elif packet_type == 3:
                    topic, payload = parse_publish(first, body)
                    try:
                        document = json.loads(payload.decode("utf-8"))
                    except (UnicodeDecodeError, json.JSONDecodeError):
                        continue
                    for event in find_ble_events(document):
                        print_event(topic, event)
        except KeyboardInterrupt:
            print("\nStopped.")
            return
        except Exception as exc:
            print(f"MQTT error: {exc}; reconnecting in {reconnect:g}s")
            time.sleep(reconnect)
        finally:
            if sock is not None:
                sock.close()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="192.168.10.41")
    parser.add_argument("--port", type=int, default=1883)
    parser.add_argument("--topic", action="append", dest="topics")
    parser.add_argument("--reconnect", type=float, default=3.0)
    args = parser.parse_args()
    run(args.host, args.port, tuple(args.topics or DEFAULT_TOPICS), args.reconnect)


if __name__ == "__main__":
    main()
