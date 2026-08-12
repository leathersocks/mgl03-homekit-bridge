# MGL03 HomeKit BLE bridge

[한국어](README.md) · [Changelog](CHANGELOG.en.md)

A small, Home Assistant-free bridge that runs directly on Xiaomi Gateway 3
(`lumi.gateway.mgl03`). It converts BLE reports from the currently supported
`miaomiaoce.sensor_ht.o2` / LYWSD02MMC sensor into native HomeKit temperature,
humidity, and battery services.

```text
miaomiaoce.sensor_ht.o2
          │ BLE
          ▼
Xiaomi Gateway 3 → openmiio_agent → local MQTT → this bridge → Apple Home
```

## What works

- Exact Xiaomi product match: `pdid=5860`, model `miaomiaoce.sensor_ht.o2`.
- Temperature, humidity, battery level, and low-battery status.
- Automatic enrollment of supported sensors during a 30-second window on a new installation.
- Persistent sensor identity and HomeKit pairing across restarts.
- Duplicate/out-of-order BLE filtering and MQTT/sensor offline status.
- Native Linux MIPSLE/soft-float binary for MGL03.
- No Home Assistant, Python runtime, Node.js runtime, or external MQTT library.

The gateway acts as a HomeKit **accessory bridge**, not as Apple's Home hub.
Local control from Apple Home works on the same LAN. Remote access and Home
automations still require an Apple TV or HomePod configured as a home hub.

## Build

Go 1.22 or newer. The release binary for the 58 MiB MGL03 is built with Go
1.25.12 and `GOMIPS=softfloat`:

```powershell
go test ./...
./scripts/build.ps1 -Version 0.1.0
```

The MGL03 binary is written to `bin/mgl03-homekit-bridge`. Linux/macOS users can
run `make test build-mgl03`.

## Install

See [docs/INSTALL.en.md](docs/INSTALL.en.md). At a high level:

1. Build the MIPSLE bridge binary.
2. On MGL03 firmware `1.5.0` through `1.5.4`, run the no-Telnet installer from
   a PC on the same LAN and enter the 32-character miIO token at its hidden
   prompt:

   ```powershell
   py -m pip install -r requirements-installer.txt
   py .\scripts\install_no_telnet.py --gateway-ip 192.168.10.41
   ```

3. Wait for the sensor's next BLE advertisement.
4. Enter the random pairing code in Apple Home on a fresh installation.

The installer uses the firmware's local miIO `set_ip_info` path to make the
gateway pull a credential-free bundle from a temporary HTTP server on the PC.
Artifacts are verified with SHA-256, with MD5 retained only as a compatibility
fallback for BusyBox builds without `sha256sum`. It never opens a Telnet session
or enables TCP port 23.
Existing `/data/mgl03-homekit` configuration, sensors, and HomeKit pairing are
preserved during updates. See the installation guide for firmware limitations,
rollback behavior, and the manual fallback.

To provide only openmiio/MQTT to an external consumer such as SmartThings Edge,
without running the HomeKit bridge, use this mode; no bridge build is required:

```powershell
py .\scripts\install_no_telnet.py `
  --gateway-ip 192.168.10.41 `
  --mode openmiio
```

`openmiio` mode installs only `/data/openmiio_agent`, the shared runtime start
script, and the boot hook, then verifies TCP `1883`. It does not create or
change the HomeKit binary, configuration, pairing data, or TCP `51826`. The
default `homekit` mode continues to install the complete bridge. Switching an
existing full installation to `openmiio` mode stops the HomeKit process while
preserving its configuration and pairing data.

To inspect MQTT BLE events only, run the diagnostic tool on the PC. It
subscribes only to `miio/report` and `central/report` by default:

```powershell
py .\scripts\mqtt_ble_probe.py --host 192.168.10.41
```

The first run creates a secure random pairing PIN and discovers matching sensors
for 30 seconds. The PIN is logged only when a new configuration is created, in a
permission-restricted log. An editable example is available at
[configs/config.example.json](configs/config.example.json).

`discovery.mode` accepts `auto`, `first`, or `manual`. `auto` enrolls multiple
sensors during the startup window and records newly observed supported sensors
for exposure after the next bridge restart. Configurations created by older
versions retain the legacy `first` behavior. During a temporary MQTT disconnect,
HomeKit retains the last accepted sensor values while the bridge reconnects in
the background.

## Design notes

`openmiio_agent` publishes the MGL03 Bluetooth service's JSON to
`central/report` or `miio/report`, depending on the stock firmware path. The
bridge subscribes to both topics, accepts only PDID 5860 BLE events, and decodes
the verified XiaomiGateway3 mappings. MIoT property updates are also accepted
after the device DID is known. Details and sample payloads are in
[docs/PROTOCOL.en.md](docs/PROTOCOL.en.md).

HomeKit is implemented with the lightweight
[`github.com/brutella/hap`](https://github.com/brutella/hap) library. The first
HomeKit accessory is a bridge and the sensor keeps a stable accessory ID derived
from its MAC address. The bundled dnssd v1.2.14 patch enables address reuse on
Linux so the bridge can share mDNS port 5353 with the MGL03 stock HomeKit
service; the stock service does not need to be stopped.

## Security and recovery

- openmiio's unauthenticated MQTT port 1883 is reachable from the LAN. Keep the
  gateway on a trusted IoT VLAN and never forward that port; the bridge itself
  connects only through `127.0.0.1`.
- Back up `/data/mgl03-homekit` before firmware or bridge upgrades.
- Firmware updates may remove the custom startup hook or disable the local miIO
  installation path.
- The no-Telnet installer is intentionally limited to MGL03 firmware
  `1.5.0`-`1.5.4`; it refuses other models and versions.
- The project does not patch the read-only firmware.

## Upstream references

- [openmiio_agent](https://github.com/AlexxIT/openmiio_agent)
- [XiaomiGateway3](https://github.com/AlexxIT/XiaomiGateway3)
- [brutella/hap](https://github.com/brutella/hap)
