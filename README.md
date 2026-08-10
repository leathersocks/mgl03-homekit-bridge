# MGL03 HomeKit BLE bridge

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
- Automatic first-sensor discovery; no MAC address is needed in advance.
- Persistent sensor identity and HomeKit pairing across restarts.
- Native Linux MIPSLE/soft-float binary for MGL03.
- No Home Assistant, Python runtime, Node.js runtime, or external MQTT library.

The gateway acts as a HomeKit **accessory bridge**, not as Apple's Home hub.
Local control from Apple Home works on the same LAN. Remote access and Home
automations still require an Apple TV or HomePod configured as a home hub.

## Build

Go 1.22 or newer:

```powershell
go test ./...
./scripts/build.ps1 -Version 0.1.0
```

The MGL03 binary is written to `bin/mgl03-homekit-bridge`. Linux/macOS users can
run `make test build-mgl03`.

## Install

See [docs/INSTALL.md](docs/INSTALL.md). At a high level:

1. Enable shell access on the MGL03 using the method compatible with its exact
   firmware.
2. Put `openmiio_agent`, the bridge binary, and the supplied start script in
   `/data`.
3. Start the service and wait for the sensor's next BLE advertisement.
4. Enter the random pairing code shown in the bridge log in Apple Home.

The first run creates a secure random pairing PIN and discovers the first
matching sensor automatically. An editable example is available at
[configs/config.example.json](configs/config.example.json).

## Design notes

`openmiio_agent` publishes the MGL03 central service's JSON to
`central/report`. The bridge accepts only PDID 5860 BLE events and decodes the
verified XiaomiGateway3 mappings. MIoT property updates are also accepted after
the device DID is known. Details and sample payloads are in
[docs/PROTOCOL.md](docs/PROTOCOL.md).

HomeKit is implemented with the lightweight
[`github.com/brutella/hap`](https://github.com/brutella/hap) library. The first
HomeKit accessory is a bridge and the sensor keeps a stable accessory ID derived
from its MAC address.

## Security and recovery

- Keep MQTT bound to the gateway/LAN; never forward port 1883 from the router.
- Back up `/data/mgl03-homekit` before firmware or bridge upgrades.
- Firmware updates may disable shell access or remove a custom startup hook.
- This project does not patch firmware and does not include an unlock exploit.

## Upstream references

- [openmiio_agent](https://github.com/AlexxIT/openmiio_agent)
- [XiaomiGateway3](https://github.com/AlexxIT/XiaomiGateway3)
- [brutella/hap](https://github.com/brutella/hap)
