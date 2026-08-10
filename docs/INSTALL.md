# MGL03 installation

> This is a community integration. It is not Apple-certified and modifies the
> software running on the gateway. Keep a recovery path for your MGL03 and do
> not expose the openmiio MQTT port to the internet.

## Requirements

- Xiaomi Gateway 3, model `lumi.gateway.mgl03`, with a working shell/telnet
  access method appropriate for its firmware.
- `openmiio_agent` MIPS binary at `/data/openmiio_agent`.
- `miaomiaoce.sensor_ht.o2` / LYWSD02MMC already visible to the gateway over
  Bluetooth.
- iPhone/iPad on the same LAN for initial HomeKit pairing.

## Copy files

Build the project on a computer, then copy these files to the gateway:

| Local file | MGL03 path |
|---|---|
| `bin/mgl03-homekit-bridge` | `/data/mgl03-homekit-bridge` |
| `scripts/start.sh` | `/data/mgl03-homekit-start.sh` |
| `scripts/stop.sh` | `/data/mgl03-homekit-stop.sh` |

On the gateway:

```sh
chmod 755 /data/openmiio_agent /data/mgl03-homekit-bridge
chmod 755 /data/mgl03-homekit-start.sh /data/mgl03-homekit-stop.sh
/data/mgl03-homekit-start.sh
tail -f /data/mgl03-homekit/bridge.log
```

The first start creates `/data/mgl03-homekit/config.json` with a random pairing
PIN, then waits for the first `miaomiaoce.sensor_ht.o2` advertisement. The log
shows both the discovery and a pairing code in `XXX-XX-XXX` form. In Apple Home,
choose **Add Accessory → More Options**, select **MGL03 Bluetooth Bridge**, and
enter that code.

The discovered MAC and DID are saved in `/data/mgl03-homekit/devices.json`.
Pairing keys are kept in `/data/mgl03-homekit/hap`; preserve both locations
across reboots and upgrades.

## More than one sensor

Automatic discovery registers only the first matching sensor. For additional
sensors, stop the bridge and add their MAC/DID entries to `config.json` using
`configs/config.example.json` as a guide. Remove `devices.json` only if you
intentionally want to rediscover the first sensor.

## Autostart

Firmware-specific telnet unlock methods install different boot hooks. Add this
single command to the persistent startup hook supplied by your unlock method:

```sh
/data/mgl03-homekit-start.sh
```

Do not edit a stock startup file blindly: Xiaomi firmware updates can replace
it and different MGL03 releases use different layouts.

## Reset HomeKit pairing

Stop the bridge, make a backup, then remove only
`/data/mgl03-homekit/hap`. Start the bridge and pair again. Keep
`devices.json` unless sensor discovery must also be reset.
