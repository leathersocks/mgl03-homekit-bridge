package bridge

import (
	"fmt"

	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/service"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/config"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/openmiio"
)

// DeviceAccessory is the product-independent runtime contract used by the
// HomeKit bridge. New Xiaomi BLE families add a decoder to openmiio's product
// registry and one constructor to accessoryFactories.
type DeviceAccessory interface {
	DeviceConfig() config.Device
	HAPAccessory() *accessory.A
	Apply(openmiio.Update)
	Close()
}

type accessoryFactory func(config.Device) DeviceAccessory

var accessoryFactories = map[openmiio.ProductKind]accessoryFactory{
	openmiio.ProductKindClimate: func(device config.Device) DeviceAccessory {
		return NewSensor(device)
	},
	openmiio.ProductKindToothbrush: func(device config.Device) DeviceAccessory {
		return NewToothbrush(device)
	},
}

func NewDeviceAccessory(device config.Device) (DeviceAccessory, error) {
	product, ok := productForDevice(device)
	if !ok {
		return nil, fmt.Errorf("unsupported BLE device: product_id=%d model=%q", device.ProductID, device.Model)
	}
	factory := accessoryFactories[product.Kind]
	if factory == nil {
		return nil, fmt.Errorf("no HomeKit accessory for BLE product kind %q", product.Kind)
	}
	return factory(device), nil
}

func productForDevice(device config.Device) (openmiio.Product, bool) {
	if product, ok := openmiio.LookupProduct(device.ProductID); ok {
		return product, true
	}
	return openmiio.LookupProductByModel(device.Model)
}

func accessoryInfo(device config.Device, fallbackModel, fallbackManufacturer string) accessory.Info {
	name := device.Name
	model := device.Model
	manufacturer := fallbackManufacturer
	if product, ok := productForDevice(device); ok {
		if name == "" {
			name = product.DefaultName
		}
		if model == "" {
			model = product.Model
		}
		if product.Manufacturer != "" {
			manufacturer = product.Manufacturer
		}
	}
	if model == "" {
		model = fallbackModel
	}
	return accessory.Info{
		Name:         name,
		SerialNumber: serialNumber(device),
		Manufacturer: manufacturer,
		Model:        model,
		Firmware:     "MGL03 bridge",
	}
}

func applyBattery(service *service.BatteryService, battery int) {
	service.BatteryLevel.SetValue(battery)
	low := 0
	if battery <= 20 {
		low = 1
	}
	service.StatusLowBattery.SetValue(low)
}
