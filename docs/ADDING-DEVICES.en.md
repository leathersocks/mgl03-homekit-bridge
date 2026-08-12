# Adding Xiaomi BLE products

[한국어](ADDING-DEVICES.md)

The bridge separates transport parsing, product decoding, and HomeKit
presentation. A new product should not add PDID branches to the MQTT runtime.

```text
MQTT envelope
  -> product registry (PDID, model, kind)
  -> BLE decoder (EID/edata -> typed Update)
  -> accessory factory (kind -> HomeKit services)
  -> stable AID and Apple Home
```

## Extension points

1. Add the product ID, model, kind, metadata, and decoder function to
   `internal/openmiio/products.go`.
2. Add a decoder in `internal/openmiio/decoders.go`, or a separate decoder file
   for a larger family. It should validate lengths and value ranges and fill a
   typed field on `openmiio.Update`.
3. Reuse an existing `ProductKind` when the HomeKit service shape matches. For
   a new shape, add a `DeviceAccessory` implementation under `internal/bridge`
   and register its constructor in `accessoryFactories`.
4. Add real-packet parser tests, state-transition tests, duplicate/out-of-order
   tests, and stable-AID migration coverage.
5. Update both Korean and English protocol, README, and changelog files.

`config.Device` and `devices.json` are product-neutral. MAC/DID matching and
the persisted AID are shared by all product types. Legacy entries without a
product ID still migrate to PDID 5860, while a known model can resolve its PDID.

## Current product families

| Kind | Product | Decoder | HomeKit accessory |
|---|---|---|---|
| `climate` | `miaomiaoce.sensor_ht.o2`, PDID 5860 | temperature, humidity, battery | Temperature Sensor + Humidity Sensor + Battery |
| `toothbrush` | `k0918.toothbrush.t700i`, PDID 6032 | state, timestamp, score, battery | Motion Sensor + Battery |

## Design rules

- Reject unknown PDIDs before enrollment.
- Treat Xiaomi event timestamps as untrusted input and retain gateway receive
  time when session reconstruction needs it.
- Keep transport de-duplication product-independent. A product-specific
  duplicate exception must be narrow; T700i only refreshes an already active
  watchdog for an exact duplicate start frame.
- Prefer standard HomeKit services. Keep unsupported metadata in diagnostics
  instead of inventing writable switches or misleading characteristics.
- Do not change a persisted AID when adding product support. Existing Apple
  Home rooms, names, and automations depend on that identity.
- Avoid long-lived per-device goroutines. Timers must be stopped by `Close` so
  the low-memory MGL03 runtime can shut down cleanly.

## Verification checklist

```text
go test ./...
go vet ./...
python -m unittest discover -s tests -v
./scripts/build.ps1 -Version <version>
```

Also verify the MIPSLE binary remains below the CI size limit, existing climate
sensor AIDs are unchanged, a new product auto-enrolls only after a valid
decoded report, and Apple Home retains the bridge pairing after upgrade.
