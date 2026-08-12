package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/bridge"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/config"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/mqttmini"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/openmiio"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/registry"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "/data/mgl03-homekit/config.json", "configuration file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	if err := run(*configPath); err != nil {
		log.Fatal(err)
	}
}

func run(configPath string) error {
	cfg, created, err := config.LoadOrCreate(configPath)
	if err != nil {
		return err
	}
	if created {
		log.Printf("created configuration: %s", configPath)
		log.Printf("HomeKit pairing code: %s", config.FormatPin(cfg.Pin))
	} else {
		log.Printf("HomeKit pairing code is stored in the private configuration file")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	updates := make(chan openmiio.Update, 128)
	mqttStates := make(chan bool, 1)
	setMQTTState := func(connected bool) {
		select {
		case mqttStates <- connected:
		default:
			select {
			case <-mqttStates:
			default:
			}
			mqttStates <- connected
		}
	}
	droppedUpdates := 0
	lastDropLog := time.Time{}
	mqtt := mqttmini.Client{
		Address:  cfg.MQTT.Address,
		Topics:   []string{cfg.MQTT.Topic, "miio/report", "openmiio/report"},
		ClientID: cfg.MQTT.ClientID,
		OnError:  func(err error) { log.Printf("MQTT: %v; reconnecting", err) },
		OnState:  setMQTTState,
	}
	go mqtt.Run(ctx, func(payload []byte) {
		parsed, parseErr := openmiio.Parse(payload)
		if parseErr != nil {
			log.Printf("openmiio message: %v", parseErr)
		}
		for _, update := range parsed {
			select {
			case updates <- update:
			case <-ctx.Done():
				return
			default:
				droppedUpdates++
				if lastDropLog.IsZero() || time.Since(lastDropLog) >= time.Minute {
					log.Printf("update queue full; dropped %d report(s), latest DID %s", droppedUpdates, update.DID)
					droppedUpdates = 0
					lastDropLog = time.Now()
				}
			}
		}
	})

	registryPath := filepath.Join(cfg.StateDir, "devices.json")
	saved, err := registry.Load(registryPath)
	if err != nil {
		return err
	}
	devices := mergeDevices(cfg.Devices, saved)
	devices, migrated := prepareDevices(devices)
	deduplicator := openmiio.NewDeduplicator(10 * time.Minute)
	initialUpdates := make(map[string]openmiio.Update)
	var discovered bool
	devices, initialUpdates, discovered, err = discoverDevices(ctx, cfg, updates, deduplicator, devices)
	if err != nil {
		return err
	}
	if migrated || discovered {
		if err := registry.Save(registryPath, devices); err != nil {
			return fmt.Errorf("save device registry: %w", err)
		}
	}

	sort.Slice(devices, func(i, j int) bool { return deviceKey(devices[i]) < deviceKey(devices[j]) })
	bridgeAccessory := accessory.NewBridge(accessory.Info{
		Name:         cfg.BridgeName,
		SerialNumber: "MGL03-BLE-BRIDGE",
		Manufacturer: "Community",
		Model:        "lumi.gateway.mgl03",
		Firmware:     version,
	})
	bridgeAccessory.A.Id = 1

	sensors := make([]*bridge.Sensor, 0, len(devices))
	accessories := make([]*accessory.A, 0, len(devices))
	for _, device := range devices {
		sensor := bridge.NewSensor(device)
		sensors = append(sensors, sensor)
		accessories = append(accessories, sensor.Accessory)
	}

	go processUpdates(ctx, updates, mqttStates, sensors, deduplicator, initialUpdates, cfg, registryPath, devices)

	hapDir := filepath.Join(cfg.StateDir, "hap")
	if err := os.MkdirAll(hapDir, 0o700); err != nil {
		return fmt.Errorf("create HomeKit state directory: %w", err)
	}
	server, err := hap.NewServer(hap.NewFsStore(hapDir), bridgeAccessory.A, accessories...)
	if err != nil {
		return fmt.Errorf("create HomeKit server: %w", err)
	}
	server.Pin = cfg.Pin
	server.Addr = fmt.Sprintf(":%d", cfg.Port)
	server.Ifaces = cfg.Interfaces
	log.Printf("HomeKit bridge %q is ready on port %d with %d sensor(s)", cfg.BridgeName, cfg.Port, len(sensors))
	if err := server.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
		return fmt.Errorf("serve HomeKit: %w", err)
	}
	return nil
}

func processUpdates(
	ctx context.Context,
	updates <-chan openmiio.Update,
	mqttStates <-chan bool,
	sensors []*bridge.Sensor,
	deduplicator *openmiio.Deduplicator,
	initial map[string]openmiio.Update,
	cfg config.Config,
	registryPath string,
	devices []config.Device,
) {
	unknown := make(map[string]bool)
	lastSeen := make(map[*bridge.Sensor]time.Time, len(sensors))
	for key, update := range initial {
		if sensor := matchSensor(sensors, update); sensor != nil {
			sensor.Apply(update)
			lastSeen[sensor] = time.Now()
		}
		delete(initial, key)
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	mqttConnected := false
	for {
		select {
		case <-ctx.Done():
			return
		case connected := <-mqttStates:
			mqttConnected = connected
			if !connected {
				for _, sensor := range sensors {
					sensor.MarkActive(false)
				}
			}
		case now := <-ticker.C:
			if !mqttConnected {
				continue
			}
			offlineAfter := time.Duration(cfg.SensorOfflineSeconds) * time.Second
			for _, sensor := range sensors {
				seenAt := lastSeen[sensor]
				if seenAt.IsZero() || now.Sub(seenAt) >= offlineAfter {
					sensor.MarkActive(false)
				}
			}
		case update := <-updates:
			if !deduplicator.Accept(update) {
				continue
			}
			sensor := matchSensor(sensors, update)
			if sensor == nil {
				key := update.MAC
				if key == "" {
					key = update.DID
				}
				if supportedDiscoveryUpdate(update) && !unknown[key] {
					unknown[key] = true
					if cfg.Discovery.Mode == config.DiscoveryModeAuto {
						device := deviceFromUpdate(update, len(devices)+1)
						devices = appendDeviceWithAID(devices, device)
						if err := registry.Save(registryPath, devices); err != nil {
							log.Printf("auto-enroll %s: %v", key, err)
						} else {
							log.Printf("auto-enrolled %s; restart the bridge to expose it to HomeKit", key)
						}
					} else {
						log.Printf("ignoring additional unconfigured %s: %s", openmiio.ModelSensorHTO2, key)
					}
				}
				continue
			}
			sensor.Apply(update)
			lastSeen[sensor] = time.Now()
		}
	}
}

func discoverDevices(
	ctx context.Context,
	cfg config.Config,
	updates <-chan openmiio.Update,
	deduplicator *openmiio.Deduplicator,
	devices []config.Device,
) ([]config.Device, map[string]openmiio.Update, bool, error) {
	initial := make(map[string]openmiio.Update)
	if len(devices) > 0 || cfg.Discovery.Mode == config.DiscoveryModeManual {
		return devices, initial, false, nil
	}

	log.Printf("waiting to discover %s (PDID %d)", openmiio.ModelSensorHTO2, openmiio.ProductIDSensorHTO2)
	var timer *time.Timer
	var timerC <-chan time.Time
	discovered := false
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return devices, initial, discovered, nil
		case <-timerC:
			return devices, initial, discovered, nil
		case update := <-updates:
			if !supportedDiscoveryUpdate(update) || !deduplicator.Accept(update) {
				continue
			}
			key := updateIdentity(update)
			initial[key] = update
			if findDevice(devices, update) >= 0 {
				continue
			}
			device := deviceFromUpdate(update, len(devices)+1)
			devices = appendDeviceWithAID(devices, device)
			discovered = true
			log.Printf("discovered %s at %s", openmiio.ModelSensorHTO2, update.MAC)
			if cfg.Discovery.Mode == config.DiscoveryModeFirst {
				return devices, initial, true, nil
			}
			if timer == nil {
				timer = time.NewTimer(time.Duration(cfg.Discovery.WindowSeconds) * time.Second)
				timerC = timer.C
				log.Printf("collecting additional supported sensors for %d seconds", cfg.Discovery.WindowSeconds)
			}
		}
	}
}

func supportedDiscoveryUpdate(update openmiio.Update) bool {
	return openmiio.SupportedProduct(update.ProductID) && update.MAC != ""
}

func deviceFromUpdate(update openmiio.Update, index int) config.Device {
	product, _ := openmiio.LookupProduct(update.ProductID)
	name := product.DefaultName
	if name == "" {
		name = "Bluetooth Sensor"
	}
	return config.Device{
		Name:      fmt.Sprintf("%s %d", name, index),
		MAC:       update.MAC,
		DID:       update.DID,
		ProductID: update.ProductID,
		Model:     update.Model,
	}
}

func prepareDevices(devices []config.Device) ([]config.Device, bool) {
	prepared := append([]config.Device(nil), devices...)
	changed := false
	used := make(map[uint64]bool, len(prepared))
	for i := range prepared {
		if prepared[i].ProductID == 0 {
			prepared[i].ProductID = openmiio.ProductIDSensorHTO2
			changed = true
		}
		if prepared[i].Model == "" {
			prepared[i].Model = openmiio.ModelSensorHTO2
			changed = true
		}
		if prepared[i].AID >= 2 && !used[prepared[i].AID] {
			used[prepared[i].AID] = true
			continue
		}
		candidate := bridge.DefaultAccessoryID(prepared[i])
		for candidate < 2 || used[candidate] {
			candidate++
			if candidate > 0x7fffffff {
				candidate = 2
			}
		}
		prepared[i].AID = candidate
		used[candidate] = true
		changed = true
	}
	return prepared, changed
}

func appendDeviceWithAID(devices []config.Device, device config.Device) []config.Device {
	all := append(append([]config.Device(nil), devices...), device)
	prepared, _ := prepareDevices(all)
	return prepared
}

func findDevice(devices []config.Device, update openmiio.Update) int {
	for i, device := range devices {
		if update.MAC != "" && device.MAC != "" && strings.EqualFold(update.MAC, device.MAC) {
			return i
		}
		if update.DID != "" && device.DID != "" && update.DID == device.DID {
			return i
		}
	}
	return -1
}

func updateIdentity(update openmiio.Update) string {
	if update.MAC != "" {
		return strings.ToLower(update.MAC)
	}
	return "did:" + update.DID
}

func matchSensor(sensors []*bridge.Sensor, update openmiio.Update) *bridge.Sensor {
	for _, sensor := range sensors {
		if update.MAC != "" && sensor.Device.MAC != "" && strings.EqualFold(update.MAC, sensor.Device.MAC) {
			return sensor
		}
		if update.DID != "" && sensor.Device.DID != "" && update.DID == sensor.Device.DID {
			return sensor
		}
	}
	return nil
}

func mergeDevices(configured, saved []config.Device) []config.Device {
	merged := append([]config.Device(nil), configured...)
	seenMAC := make(map[string]bool, len(merged))
	seenDID := make(map[string]bool, len(merged))
	for _, device := range merged {
		if device.MAC != "" {
			seenMAC[strings.ToLower(device.MAC)] = true
		}
		if device.DID != "" {
			seenDID[device.DID] = true
		}
	}
	for _, device := range saved {
		duplicate := device.MAC != "" && seenMAC[strings.ToLower(device.MAC)]
		duplicate = duplicate || device.DID != "" && seenDID[device.DID]
		if duplicate || deviceKey(device) == "" {
			continue
		}
		if device.MAC != "" {
			seenMAC[strings.ToLower(device.MAC)] = true
		}
		if device.DID != "" {
			seenDID[device.DID] = true
		}
		merged = append(merged, device)
	}
	return merged
}

func deviceKey(device config.Device) string {
	if device.MAC != "" {
		return strings.ToLower(device.MAC)
	}
	if device.DID != "" {
		return "did:" + device.DID
	}
	return ""
}
