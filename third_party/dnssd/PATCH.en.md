# Local dnssd patch

[한국어](PATCH.md)

This directory contains `github.com/brutella/dnssd` v1.2.14 under its MIT
license. The Linux mDNS listener is patched to set `SO_REUSEADDR` before bind
and opportunistically set `SO_REUSEPORT`.

The stock Xiaomi MGL03 HomeKit service already listens on UDP port 5353. Socket
reuse lets the Bluetooth HomeKit bridge coexist with that service instead of
requiring the stock service to be stopped.
