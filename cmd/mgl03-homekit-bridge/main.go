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
	}
	log.Printf("HomeKit pairing code: %s", config.FormatPin(cfg.Pin))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	updates := make(chan openmiio.Update, 64)
	mqtt := mqttmini.Client{
		Address:  cfg.MQTT.Address,
		Topics:   []string{cfg.MQTT.Topic, "miio/report"},
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
			}
		}
	})

	registryPath := filepath.Join(cfg.StateDir, "devices.json")
	saved, err := registry.Load(registryPath)
	if err != nil {
		return err
	}
	devices := mergeDevices(cfg.Devices, saved)
	var firstUpdate *openmiio.Update
	if len(devices) == 0 {
		log.Printf("waiting to discover %s (PDID %d)", openmiio.ModelSensorHTO2, openmiio.ProductIDSensorHTO2)
		for len(devices) == 0 {
			select {
			case <-ctx.Done():
				return nil
			case update := <-updates:
				if update.ProductID != openmiio.ProductIDSensorHTO2 || update.MAC == "" {
					continue
				}
				device := config.Device{Name: "Miaomiaoce Temperature Sensor", MAC: update.MAC, DID: update.DID}
				devices = []config.Device{device}
				firstUpdate = &update
				if err := registry.Save(registryPath, devices); err != nil {
					return fmt.Errorf("save discovered sensor: %w", err)
				}
				log.Printf("discovered %s at %s", openmiio.ModelSensorHTO2, update.MAC)
			}
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

	if firstUpdate != nil {
		if sensor := matchSensor(sensors, *firstUpdate); sensor != nil {
			sensor.Apply(*firstUpdate)
		}
	}

	go processUpdates(ctx, updates, sensors)

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

func processUpdates(ctx context.Context, updates <-chan openmiio.Update, sensors []*bridge.Sensor) {
	unknown := make(map[string]bool)
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			sensor := matchSensor(sensors, update)
			if sensor == nil {
				key := update.MAC
				if key == "" {
					key = update.DID
				}
				if update.ProductID == openmiio.ProductIDSensorHTO2 && !unknown[key] {
					unknown[key] = true
					log.Printf("ignoring additional unconfigured %s: %s", openmiio.ModelSensorHTO2, key)
				}
				continue
			}
			sensor.Apply(update)
		}
	}
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
