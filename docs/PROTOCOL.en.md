# `miaomiaoce.sensor_ht.o2` protocol mapping

[한국어](PROTOCOL.md)

This bridge intentionally supports one Xiaomi BLE product family:

| Field | Value |
|---|---|
| Xiaomi model | `miaomiaoce.sensor_ht.o2` |
| Market model | `LYWSD02MMC` |
| Product ID (`pdid`) | `5860` |
| Transport | MiBeacon v2 event or MIoT property report |

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

Only BLE events with `pdid=5860` are accepted. MIoT property reports do not
contain a product ID, so they are applied only when their `did` matches an
already configured or discovered sensor.

## References

- [XiaomiGateway3 device database](https://github.com/AlexxIT/XiaomiGateway3/blob/master/custom_components/xiaomi_gateway3/core/devices.py)
- [XiaomiGateway3 BLE event handler](https://github.com/AlexxIT/XiaomiGateway3/blob/master/custom_components/xiaomi_gateway3/core/gate/ble.py)
- [openmiio_agent central proxy](https://github.com/AlexxIT/openmiio_agent/blob/master/internal/central/init.go)
