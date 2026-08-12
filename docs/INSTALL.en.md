# MGL03 installation

[한국어](INSTALL.md)

> This is a community integration. It is not Apple-certified and modifies the
> software running on the gateway. Keep a recovery path for your MGL03 and do
> not expose the openmiio MQTT port to the internet.

## Requirements

- Xiaomi Gateway 3, model `lumi.gateway.mgl03`.
- Firmware `1.5.0` through `1.5.4` and its 32-character miIO token for the
  no-Telnet installer. Other firmware requires a compatible manual access
  method.
- Python 3 and `python-miio==0.5.12` on a PC in the same LAN.
- `miaomiaoce.sensor_ht.o2` / LYWSD02MMC already visible to the gateway over
  Bluetooth.
- iPhone/iPad on the same LAN for initial HomeKit pairing.

## Install without Telnet

Build the MIPSLE bridge binary first, then install the PC-side dependency:

```powershell
Set-Location C:\Git\mgl03-homekit-bridge
.\scripts\build.ps1 -Version dev
py -m pip install -r .\requirements-installer.txt
```

Run the installer. The token is entered at a hidden prompt and is never written
to the bundle, command line, logs, or gateway filesystem:

```powershell
py .\scripts\install_no_telnet.py --gateway-ip 192.168.10.41
```

The installer performs the following operations without opening a Telnet
session:

1. Verifies the model and supported firmware through local miIO.
2. Downloads or verifies official `openmiio_agent` v1.2.1 for MIPS.
3. Starts a temporary HTTP server containing only binaries and scripts—never
   the token, Gateway Key, HomeKit PIN, or pairing state.
4. Sends a short `set_ip_info` request that tells the gateway to pull the
   bundle.
5. Verifies every file with SHA-256 on the gateway, rejects unknown startup
   hooks, atomically replaces runtime files, and rolls back if the bridge
   fails. MD5 is used only on old BusyBox builds without `sha256sum`.
6. Receives a one-time HTTP callback with the result and verifies TCP `51826`
   when the bridge is ready.

The temporary server selects a free port automatically. Override PC address or
port only when routing or firewall policy requires it:

```powershell
py .\scripts\install_no_telnet.py `
  --gateway-ip 192.168.10.41 `
  --pc-ip 192.168.10.100 `
  --http-port 8000
```

If the PC cannot access GitHub during installation, supply a previously
downloaded official MIPS binary. Its expected SHA-256 is
`78c775b354bb5fb896682fd3c26b9114cf336387985629ca16bc40a19cfb74f6`:

```powershell
py .\scripts\install_no_telnet.py `
  --gateway-ip 192.168.10.41 `
  --openmiio-bin C:\path\to\openmiio_agent_mips
```

This path is limited to `lumi.gateway.mgl03` firmware `1.5.0`-`1.5.4`. It
refuses `1.5.5+`, which uses a different authenticated command, and never
changes firmware. TCP port `23` remains closed. Existing
`/data/mgl03-homekit/config.json`, `devices.json`, `hap`, and logs are preserved
when updating an installed bridge.

## Manual Telnet fallback

Use this only when the no-Telnet installer does not support the installed
firmware or when diagnosing a failed installation. Enable temporary shell
access with a method appropriate for the exact firmware, then copy the same
runtime files manually.

### Copy files

Build the project on a computer, then copy these files to the gateway:

| Local file | MGL03 path |
|---|---|
| `bin/mgl03-homekit-bridge` | `/data/mgl03-homekit-bridge` |
| `scripts/start.sh` | `/data/mgl03-homekit-start.sh` |
| `scripts/stop.sh` | `/data/mgl03-homekit-stop.sh` |
| `scripts/startup.sh` | `/data/scripts/startup.sh` |
| `scripts/cleanup.sh` | `/data/mgl03-homekit-cleanup.sh` |

On the gateway:

```sh
mkdir -p /data/scripts
chmod 755 /data/openmiio_agent /data/mgl03-homekit-bridge
chmod 755 /data/mgl03-homekit-start.sh /data/mgl03-homekit-stop.sh
chmod 755 /data/scripts/startup.sh
chmod 755 /data/mgl03-homekit-cleanup.sh
/data/mgl03-homekit-start.sh
tail -f /data/mgl03-homekit/bridge.log
```

The original MGL03 has only about 58 MiB of usable RAM and no swap. The supplied
start script therefore runs the bridge with one OS thread, a 16 MiB Go memory
limit, and more frequent garbage collection. These defaults can be overridden
by setting `GOMAXPROCS`, `GOMEMLIMIT`, or `GOGC` before invoking the script.

The script scans `/proc/[0-9]*/cmdline` directly, including on the stock BusyBox
image where `pidof` is absent, so an `openmiio_agent` missing the required
`miio central mqtt cache` arguments is not mistaken for a healthy process. It
returns a failure if either daemon exits during startup.

The first start creates `/data/mgl03-homekit/config.json` with a random pairing
PIN, then collects `miaomiaoce.sensor_ht.o2` advertisements for 30 seconds. The
pairing code is logged only when the configuration is created, in a
permission-restricted log. In Apple Home,
choose **Add Accessory → More Options**, select **MGL03 Bluetooth Bridge**, and
enter that code.

The discovered MAC, DID, product ID, and stable HomeKit AID are saved in
`/data/mgl03-homekit/devices.json`.
Pairing keys are kept in `/data/mgl03-homekit/hap`; preserve both locations
across reboots and upgrades.

## More than one sensor

New configurations default to `discovery.mode=auto`. Multiple sensors are
enrolled during the startup window; supported sensors first observed while the
bridge is running are saved to `devices.json` and appear in HomeKit after one
bridge restart. Existing configurations retain the legacy `first` behavior.
Set `discovery.mode` to `manual` to disable automatic enrollment.

## Autostart

The stock `1.5.0_0026` firmware has a read-only SquashFS root but its
`/etc/init.d/rcS` checks `/data/scripts/startup.sh`. When that custom file is
executable, it replaces the normal `/bin/startup.sh` command. The supplied
wrapper schedules the bridge first, waits 30 seconds for the normal gateway
services, and then transfers control to the stock command. This order matters
because `/bin/startup.sh` remains resident instead of returning. HomeKit startup
output is written to `/data/mgl03-homekit/startup.log`.

Do not modify `/etc/init.d/rcS`. Copy the wrapper to the writable `/data`
partition instead:

```sh
mkdir -p /data/scripts
chmod 755 /data/scripts/startup.sh
```

Only install the supplied wrapper when `/data/scripts/startup.sh` is absent. If
that path already belongs to another customization, preserve its existing
stock-startup handling and merge only the following asynchronous call instead
of overwriting it:

```sh
(
    sleep 30
    /data/mgl03-homekit-start.sh
) >>/data/mgl03-homekit/startup.log 2>&1 &
```

After rebooting, allow about one minute and verify both the boot wrapper and
the bridge:

```sh
cat /data/mgl03-homekit/startup.log
cat /data/mgl03-homekit/bridge.log
ps | grep '[m]gl03-homekit-bridge'
netstat -lnt | grep 51826
```

Firmware updates can replace the boot layout or disable shell access. Recheck
the hook after an update before relying on it.

## Remove installation leftovers

The cleanup script has an exact allowlist for obsolete test binaries, staging
files, and superseded pairing backups created during MGL03 bring-up. It refuses
to run if the live bridge, startup scripts, current configuration, device
registry, or HomeKit state is missing. The first run is always a dry run:

```sh
/data/mgl03-homekit-cleanup.sh
```

Review the reported paths, then apply the cleanup:

```sh
/data/mgl03-homekit-cleanup.sh --apply
```

The pre-multisensor device registry backup is retained by default. Remove it
only after confirming all configured sensors work:

```sh
/data/mgl03-homekit-cleanup.sh --apply --include-recovery
```

Current binaries, pairing state, three-sensor registry, PID file, and active
bridge/openmiio/startup logs are never cleanup targets.

## Reset HomeKit pairing

Stop the bridge, make a backup, then remove only
`/data/mgl03-homekit/hap`. Start the bridge and pair again. Keep
`devices.json` unless sensor discovery must also be reset.
