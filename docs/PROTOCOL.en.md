# Supported Xiaomi BLE protocol mappings

[한국어](PROTOCOL.md)

Product metadata and BLE decoders are kept in an internal registry so another
device family can be added without spreading PDID checks through the runtime.

| Xiaomi model | Market model | PDID | HomeKit services |
|---|---|---:|---|
| `miaomiaoce.sensor_ht.o2` | LYWSD02MMC | `5860` | temperature, humidity, battery |
| `k0918.toothbrush.t700i` | MES604 / T700i | `6032` | motion (brushing), battery |

## openmiio input

`openmiio_agent miio central mqtt cache` proxies messages from
`/tmp/central_service_lite.socket` and publishes them unchanged to the local
MQTT topic `central/report`.

A typical BLE report has this shape:

```json
{
  "method": "_async.ble_event",
  "params": {
    "dev": {
      "did": "blt.3.example",
      "mac": "AA:BB:CC:DD:EE:FF",
      "pdid": 5860
    },
    "evt": [
      { "eid": 19457, "edata": "3333bb41" },
      { "eid": 19458, "edata": "2d" },
      { "eid": 18435, "edata": "58" }
    ],
    "frmCnt": 36
  }
}
```

## Measurements

| Measurement | MiBeacon event | Encoding | MIoT fallback |
|---|---:|---|---|
| Temperature | `19457` (`0x4C01`) | IEEE-754 float32, little-endian; rounded to 0.1 °C | `siid=3`, `piid=1001` |
| Humidity | `19458` (`0x4C02`) | unsigned byte, percent | `siid=3`, `piid=1002` |
| Battery | `18435` (`0x4803`) | unsigned byte, percent | `siid=2`, `piid=1003` |

## T700i brushing event

The T700i state event uses EID `12291` (`0x3003`). Its payload contains an event
type byte, a four-byte little-endian Unix timestamp, and an optional score byte.
Standard battery reports use EID `4106` (`0x100A`).

```text
start:       00 9b 5d 77 6a       type=0, timestamp=1786207643
forced stop: 01 b9 20 77 6a       type=1, stale previous-session timestamp
battery:     64                   100 percent
```

`type=0` starts or refreshes a brushing session and any non-zero type is an end
candidate. Embedded timestamps within 60 seconds of `gwts` are treated as live.
Some real T700i forced-stop packets reuse the previous completed session's
timestamp; while a current session is active, the bridge uses `gwts` as the
effective end when the session is no longer than ten minutes. A 30-second
activity watchdog returns the HomeKit motion service to inactive if an end
advertisement is lost. Score and duration are retained for diagnostics but are
not exposed as non-standard HomeKit characteristics.

Only BLE events whose PDID has a registered decoder are accepted. MIoT property
reports do not contain a product ID, so they are applied only when their `did`
matches an already configured or discovered device.

The bridge tracks `frmCnt` per MAC/DID. Duplicate frames published on both MQTT
topics and clearly older frames are ignored; the counter baseline resets after
ten minutes of silence to permit sensor reboot or battery replacement. During
a temporary MQTT disconnect, the bridge keeps the last accepted HomeKit values
while reconnecting in the background. HomeKit determines unreachability from
the bridge connection itself.

An exact duplicate T700i start frame may refresh an already active watchdog,
but it cannot reset the original session start time. Older out-of-order frames
never change toothbrush state.

## Measurement restore after restart

When a value actually changes, the bridge atomically stores the last
temperature, humidity, and battery readings in
`/data/mgl03-homekit/measurements.json`. After an MGL03 restart it restores
those values before starting HomeKit and replaces them when fresh BLE reports
arrive. The file is created with mode `600`. T700i sessions and motion state
are deliberately excluded so stale activity cannot become active after a
restart.

## References

- [XiaomiGateway3 device database](https://github.com/AlexxIT/XiaomiGateway3/blob/master/custom_components/xiaomi_gateway3/core/devices.py)
- [XiaomiGateway3 BLE event handler](https://github.com/AlexxIT/XiaomiGateway3/blob/master/custom_components/xiaomi_gateway3/core/gate/ble.py)
- [openmiio_agent central proxy](https://github.com/AlexxIT/openmiio_agent/blob/master/internal/central/init.go)
