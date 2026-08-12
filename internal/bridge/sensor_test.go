package bridge

import (
	"testing"

	"github.com/brutella/hap/characteristic"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/config"
)

func TestStableID(t *testing.T) {
	a := stableID(config.Device{MAC: "aa:bb:cc:dd:ee:ff"})
	b := stableID(config.Device{MAC: "aa:bb:cc:dd:ee:ff"})
	c := stableID(config.Device{MAC: "00:11:22:33:44:55"})
	if a < 2 || a != b || a == c {
		t.Fatalf("ids: %d %d %d", a, b, c)
	}
}

func TestConfiguredAIDTakesPrecedence(t *testing.T) {
	device := config.Device{MAC: "aa:bb:cc:dd:ee:ff", AID: 42}
	if got := accessoryID(device); got != 42 {
		t.Fatalf("aid = %d", got)
	}
}

func TestSensorDoesNotExposePerServiceAvailability(t *testing.T) {
	sensor := NewSensor(config.Device{MAC: "aa:bb:cc:dd:ee:ff"})
	services := []*struct {
		name string
		c    func(string) *characteristic.C
	}{
		{name: "temperature", c: sensor.temperature.C},
		{name: "humidity", c: sensor.humidity.C},
	}
	for _, service := range services {
		if service.c(characteristic.TypeStatusActive) != nil || service.c(characteristic.TypeStatusFault) != nil {
			t.Fatalf("%s service exposes availability characteristics", service.name)
		}
	}
}
