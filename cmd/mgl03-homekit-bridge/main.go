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
	"github.com/leathersocks/mgl03-homekit-bridge/internal/measurements"
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
	droppedUpdates := 0
	lastDropLog := time.Time{}
	mqtt := mqttmini.Client{
		Address:  cfg.MQTT.Address,
		Topics:   []string{cfg.MQTT.Topic, "miio/report", "openmiio/report"},
		ClientID: cfg.MQTT.ClientID,
		OnError:  func(err error) { log.Printf("MQTT: %v; reconnecting", err) },
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
	measurementPath := filepath.Join(cfg.StateDir, "measurements.json")
	measurementState, err := measurements.Load(measurementPath)
	if err != nil {
		log.Printf("measurement cache: %v; starting without cached values", err)
		measurementState = measurements.New()
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

	deviceAccessories := make([]bridge.DeviceAccessory, 0, len(devices))
	accessories := make([]*accessory.A, 0, len(devices))
	for _, device := range devices {
		deviceAccessory, err := bridge.NewDeviceAccessory(device)
		if err != nil {
			log.Printf("ignoring unsupported configured BLE device %s: %v", deviceKey(device), err)
			continue
		}
		deviceAccessories = append(deviceAccessories, deviceAccessory)
		accessories = append(accessories, deviceAccessory.HAPAccessory())
	}
	defer func() {
		for _, deviceAccessory := range deviceAccessories {
			deviceAccessory.Close()
		}
	}()
	if restored := restoreCachedMeasurements(deviceAccessories, measurementState); restored > 0 {
		log.Printf("restored cached measurements for %d BLE device(s)", restored)
	}

	go processUpdates(
		ctx,
		updates,
		deviceAccessories,
		deduplicator,
		initialUpdates,
		cfg,
		registryPath,
		devices,
		measurementState,
		measurementPath,
	)

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
	log.Printf("HomeKit bridge %q is ready on port %d with %d BLE device(s)", cfg.BridgeName, cfg.Port, len(deviceAccessories))
	if err := server.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
		return fmt.Errorf("serve HomeKit: %w", err)
	}
	return nil
}

func processUpdates(
	ctx context.Context,
	updates <-chan openmiio.Update,
	deviceAccessories []bridge.DeviceAccessory,
	deduplicator *openmiio.Deduplicator,
	initial map[string]openmiio.Update,
	cfg config.Config,
	registryPath string,
	devices []config.Device,
	measurementState *measurements.State,
	measurementPath string,
) {
	unknown := make(map[string]bool)
	for key, update := range initial {
		if deviceAccessory := matchAccessory(deviceAccessories, update); deviceAccessory != nil {
			deviceAccessory.Apply(update)
			persistMeasurements(measurementState, measurementPath, update)
		}
		delete(initial, key)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			deviceAccessory := matchAccessory(deviceAccessories, update)
			disposition := deduplicator.Classify(update)
			if disposition != openmiio.FrameAccepted {
				if disposition == openmiio.FrameDuplicate && deviceAccessory != nil &&
					update.Toothbrush != nil && update.Toothbrush.Type == 0 {
					deviceAccessory.Apply(update)
				}
				continue
			}
			persistMeasurements(measurementState, measurementPath, update)
			if deviceAccessory == nil {
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
						log.Printf("ignoring additional unconfigured %s: %s", update.Model, key)
					}
				}
				continue
			}
			deviceAccessory.Apply(update)
		}
	}
}

func restoreCachedMeasurements(deviceAccessories []bridge.DeviceAccessory, state *measurements.State) int {
	restored := 0
	for _, deviceAccessory := range deviceAccessories {
		update, _, ok := state.Restore(deviceAccessory.DeviceConfig())
		if !ok {
			continue
		}
		deviceAccessory.Apply(update)
		restored++
	}
	return restored
}

func persistMeasurements(state *measurements.State, path string, update openmiio.Update) {
	if state == nil || path == "" || !state.Merge(update, time.Now()) {
		return
	}
	if err := state.Save(path); err != nil {
		log.Printf("save measurement cache: %v", err)
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

	log.Printf("waiting to discover supported Xiaomi BLE devices")
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
			log.Printf("discovered %s (PDID %d) at %s", update.Model, update.ProductID, update.MAC)
			if cfg.Discovery.Mode == config.DiscoveryModeFirst {
				return devices, initial, true, nil
			}
			if timer == nil {
				timer = time.NewTimer(time.Duration(cfg.Discovery.WindowSeconds) * time.Second)
				timerC = timer.C
				log.Printf("collecting additional supported BLE devices for %d seconds", cfg.Discovery.WindowSeconds)
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
			if product, ok := openmiio.LookupProductByModel(prepared[i].Model); ok {
				prepared[i].ProductID = product.ID
				changed = true
			} else if prepared[i].Model == "" {
				prepared[i].ProductID = openmiio.ProductIDSensorHTO2
				changed = true
			}
		}
		if prepared[i].Model == "" {
			if product, ok := openmiio.LookupProduct(prepared[i].ProductID); ok {
				prepared[i].Model = product.Model
				changed = true
			}
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

func matchAccessory(deviceAccessories []bridge.DeviceAccessory, update openmiio.Update) bridge.DeviceAccessory {
	for _, deviceAccessory := range deviceAccessories {
		device := deviceAccessory.DeviceConfig()
		if update.MAC != "" && device.MAC != "" && strings.EqualFold(update.MAC, device.MAC) {
			return deviceAccessory
		}
		if update.DID != "" && device.DID != "" && update.DID == device.DID {
			return deviceAccessory
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
