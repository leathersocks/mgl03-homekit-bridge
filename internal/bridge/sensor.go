package bridge

import (
	"hash/fnv"
	"strings"

	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/brutella/hap/service"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/config"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/openmiio"
)

type Sensor struct {
	Device            config.Device
	Accessory         *accessory.A
	temperature       *service.TemperatureSensor
	humidity          *service.HumiditySensor
	battery           *service.BatteryService
	temperatureActive *characteristic.StatusActive
	temperatureFault  *characteristic.StatusFault
	humidityActive    *characteristic.StatusActive
	humidityFault     *characteristic.StatusFault
}

func NewSensor(device config.Device) *Sensor {
	model := device.Model
	if model == "" {
		model = openmiio.ModelSensorHTO2
	}
	info := accessory.Info{
		Name:         device.Name,
		SerialNumber: serialNumber(device),
		Manufacturer: "Miaomiaoce",
		Model:        model,
		Firmware:     "MGL03 bridge",
	}
	thermometer := accessory.NewTemperatureSensor(info)
	humidity := service.NewHumiditySensor()
	battery := service.NewBatteryService()
	temperatureActive := characteristic.NewStatusActive()
	temperatureFault := characteristic.NewStatusFault()
	humidityActive := characteristic.NewStatusActive()
	humidityFault := characteristic.NewStatusFault()
	thermometer.TempSensor.AddC(temperatureActive.C)
	thermometer.TempSensor.AddC(temperatureFault.C)
	humidity.AddC(humidityActive.C)
	humidity.AddC(humidityFault.C)
	thermometer.A.AddS(humidity.S)
	thermometer.A.AddS(battery.S)
	thermometer.A.Id = accessoryID(device)
	_ = battery.ChargingState.SetValue(2) // non-rechargeable / not chargeable

	return &Sensor{
		Device:            device,
		Accessory:         thermometer.A,
		temperature:       thermometer.TempSensor,
		humidity:          humidity,
		battery:           battery,
		temperatureActive: temperatureActive,
		temperatureFault:  temperatureFault,
		humidityActive:    humidityActive,
		humidityFault:     humidityFault,
	}
}

func (s *Sensor) Apply(update openmiio.Update) {
	if update.Temperature != nil {
		s.temperature.CurrentTemperature.SetValue(*update.Temperature)
	}
	if update.Humidity != nil {
		s.humidity.CurrentRelativeHumidity.SetValue(*update.Humidity)
	}
	if update.Battery != nil {
		s.battery.BatteryLevel.SetValue(*update.Battery)
		low := 0
		if *update.Battery <= 20 {
			low = 1
		}
		s.battery.StatusLowBattery.SetValue(low)
	}
	s.MarkActive(true)
}

// MarkActive exposes source connectivity and stale-sensor state to HomeKit.
func (s *Sensor) MarkActive(active bool) {
	fault := characteristic.StatusFaultGeneralFault
	if active {
		fault = characteristic.StatusFaultNoFault
	}
	s.temperatureActive.SetValue(active)
	s.temperatureFault.SetValue(fault)
	s.humidityActive.SetValue(active)
	s.humidityFault.SetValue(fault)
}

func accessoryID(device config.Device) uint64 {
	if device.AID >= 2 {
		return device.AID
	}
	return stableID(device)
}

func stableID(device config.Device) uint64 {
	identity := device.MAC
	if identity == "" {
		identity = "did:" + device.DID
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(identity))
	return h.Sum64()&0x7fffffff + 2
}

// DefaultAccessoryID returns the legacy deterministic AID used before AIDs
// were persisted in devices.json. Migration code uses it to preserve existing
// HomeKit accessory identities.
func DefaultAccessoryID(device config.Device) uint64 {
	return stableID(device)
}

func serialNumber(device config.Device) string {
	if device.MAC != "" {
		return strings.ToUpper(strings.ReplaceAll(device.MAC, ":", ""))
	}
	return device.DID
}
