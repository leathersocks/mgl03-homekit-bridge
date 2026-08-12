# Changelog

[한국어](CHANGELOG.md)

All notable changes to this project are documented here. No version tags have
been published yet, so changes before the first formal release are collected
under `Unreleased`.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and formal releases will use [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- A `discovery.mode=auto` policy that enrolls multiple supported BLE sensors
  during a 30-second discovery window on new installations.
- A legacy-compatible `first` policy and a `manual` policy that disables
  automatic enrollment.
- Registry migration that persists each device's product ID, model, and stable
  HomeKit AID in `devices.json`.
- A product metadata registry for adding future BLE device support without
  spreading PDID checks through the runtime.
- Duplicate and out-of-order BLE frame filtering based on `frmCnt`, with the
  baseline reset after ten minutes without a report.
- HomeKit active and fault status driven by MQTT connectivity and sensor
  last-seen time.
- A `sensor_offline_seconds` setting, defaulting to 900 seconds.
- GitHub Actions checks for Go formatting, tests and vet, Python installer
  tests, ShellCheck, the MIPSLE build, binary size, and SHA-256 output.

### Changed

- Supported sensors first observed while the bridge is running are saved to the
  registry and exposed to HomeKit after the next bridge restart.
- Existing hash-derived AIDs are retained during the first registry migration
  so current HomeKit pairings and automations keep the same accessory IDs.
- MQTT now also subscribes to `openmiio/report`; the update queue is larger and
  queue saturation logging is rate-limited.
- Startup waits for MQTT port 1883 readiness instead of relying only on a fixed
  boot delay.
- The no-Telnet installer's HomeKit port readiness window is now 60 seconds to
  allow time for multi-sensor discovery.
- Korean and English README, installation, and protocol documentation now
  describe the new enrollment and status policies.

### Security

- Existing configurations no longer write the HomeKit pairing PIN on every
  startup; it is logged only when a new configuration is first created.
- Bridge and openmiio logs are created with mode `600` and bounded log rotation.
- No-Telnet installation artifacts use SHA-256 verification first, retaining
  MD5 only as a compatibility fallback for old BusyBox builds without
  `sha256sum`.
- The official openmiio_agent v1.2.1 MIPS binary is verified with both SHA-256
  and MD5.
- PID handling validates `/proc/<pid>/cmdline` as well as the numeric PID, which
  prevents a reused PID from identifying or stopping an unrelated process.

### Fixed

- Duplicate HomeKit updates when the same BLE report arrives through both
  `central/report` and `miio/report`.
- Sensors remaining apparently healthy after MQTT disconnects.
- Out-of-range values being accepted through the MIoT temperature, humidity,
  and battery fallback path.
- Startup mistaking an `openmiio_agent` process without the required
  `miio central mqtt cache` arguments for a healthy instance.
- The stop script removing the PID file without waiting for a graceful bridge
  shutdown.
- GitHub Actions failing to compile on Linux amd64 because
  `syscall.SO_REUSEPORT` is not defined there; socket options now use the
  architecture-specific constants from `x/sys/unix`.

## Initial development - 2026-08-10 to 2026-08-11

### Added

- A MIPSLE/soft-float HomeKit BLE bridge running directly on Xiaomi Gateway 3
  (`lumi.gateway.mgl03`).
- Temperature, humidity, battery level, and low-battery support for
  `miaomiaoce.sensor_ht.o2` / LYWSD02MMC (PDID 5860).
- A lightweight MQTT 3.1.1 client consuming `central/report` and `miio/report`.
- Persistent HomeKit pairing, sensor identity, and multi-sensor configuration.
- A dnssd address-reuse patch for sharing UDP port 5353 with the stock MGL03
  HomeKit service.
- Low-memory Go builds and start, stop, autostart, and cleanup scripts.
- Checksum-verified no-Telnet installation with automatic rollback for firmware
  1.5.0 through 1.5.4.
- Korean and English README, installation, and protocol documentation.
