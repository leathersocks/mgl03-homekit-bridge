package bridge

import (
	"hash/fnv"
	"strings"

	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/service"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/config"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/openmiio"
)

type Sensor struct {
	Device      config.Device
	Accessory   *accessory.A
	temperature *service.TemperatureSensor
	humidity    *service.HumiditySensor
	battery     *service.BatteryService
}

func NewSensor(device config.Device) *Sensor {
	info := accessory.Info{
		Name:         device.Name,
		SerialNumber: serialNumber(device),
		Manufacturer: "Miaomiaoce",
		Model:        openmiio.ModelSensorHTO2,
		Firmware:     "MGL03 bridge",
	}
	thermometer := accessory.NewTemperatureSensor(info)
	humidity := service.NewHumiditySensor()
	battery := service.NewBatteryService()
	thermometer.A.AddS(humidity.S)
	thermometer.A.AddS(battery.S)
	thermometer.A.Id = stableID(device)
	_ = battery.ChargingState.SetValue(2) // non-rechargeable / not chargeable

	return &Sensor{
		Device:      device,
		Accessory:   thermometer.A,
		temperature: thermometer.TempSensor,
		humidity:    humidity,
		battery:     battery,
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

func serialNumber(device config.Device) string {
	if device.MAC != "" {
		return strings.ToUpper(strings.ReplaceAll(device.MAC, ":", ""))
	}
	return device.DID
}
