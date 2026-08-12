package bridge

import (
	"testing"
	"time"

	"github.com/leathersocks/mgl03-homekit-bridge/internal/config"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/openmiio"
)

func TestDeviceAccessoryFactory(t *testing.T) {
	climate, err := NewDeviceAccessory(config.Device{ProductID: openmiio.ProductIDSensorHTO2})
	if err != nil {
		t.Fatal(err)
	}
	defer climate.Close()
	if _, ok := climate.(*Sensor); !ok {
		t.Fatalf("climate accessory = %T", climate)
	}

	toothbrush, err := NewDeviceAccessory(config.Device{Model: openmiio.ModelToothbrushT700i, AID: 42})
	if err != nil {
		t.Fatal(err)
	}
	defer toothbrush.Close()
	if _, ok := toothbrush.(*Toothbrush); !ok || toothbrush.HAPAccessory().Id != 42 {
		t.Fatalf("toothbrush accessory = %T aid=%d", toothbrush, toothbrush.HAPAccessory().Id)
	}
	if _, err := NewDeviceAccessory(config.Device{ProductID: 999999, Model: "unknown"}); err == nil {
		t.Fatal("unknown product was accepted")
	}
}

func TestToothbrushNormalSession(t *testing.T) {
	now := time.Unix(1786207645, 0)
	toothbrush := NewToothbrush(config.Device{Name: "T700i", MAC: "c0:b5:86:1f:e4:5f"})
	defer toothbrush.Close()
	toothbrush.now = func() time.Time { return now }
	battery := 100
	toothbrush.Apply(openmiio.Update{Battery: &battery})
	if toothbrush.battery.BatteryLevel.Value() != 100 || toothbrush.battery.StatusLowBattery.Value() != 0 {
		t.Fatalf("battery state: level=%d low=%d", toothbrush.battery.BatteryLevel.Value(), toothbrush.battery.StatusLowBattery.Value())
	}

	toothbrush.Apply(toothbrushUpdate(0, 1786207643, 1786207645, nil))
	if !toothbrush.active || !toothbrush.motion.MotionDetected.Value() || toothbrush.sessionStart != 1786207643 {
		t.Fatalf("start state: active=%t motion=%t start=%d", toothbrush.active, toothbrush.motion.MotionDetected.Value(), toothbrush.sessionStart)
	}
	firstStart := toothbrush.sessionStart
	firstGeneration := toothbrush.watchdogGeneration

	now = now.Add(10 * time.Second)
	toothbrush.Apply(toothbrushUpdate(0, 1786207653, 1786207655, nil))
	if toothbrush.sessionStart != firstStart || toothbrush.watchdogGeneration <= firstGeneration {
		t.Fatalf("heartbeat reset session: start=%d generation=%d", toothbrush.sessionStart, toothbrush.watchdogGeneration)
	}

	score := 95
	toothbrush.Apply(toothbrushUpdate(1, 1786207663, 1786207664, &score))
	if toothbrush.active || toothbrush.motion.MotionDetected.Value() {
		t.Fatal("normal end did not become inactive")
	}
	if toothbrush.lastDuration != 20*time.Second || toothbrush.lastScore == nil || *toothbrush.lastScore != 95 {
		t.Fatalf("completion metadata: duration=%s score=%v", toothbrush.lastDuration, toothbrush.lastScore)
	}
}

func TestToothbrushForcedStopUsesGatewayTime(t *testing.T) {
	now := time.Unix(1786207645, 0)
	toothbrush := NewToothbrush(config.Device{Name: "T700i"})
	defer toothbrush.Close()
	toothbrush.now = func() time.Time { return now }

	toothbrush.Apply(toothbrushUpdate(0, 1786207643, 1786207645, nil))
	now = now.Add(4 * time.Second)
	toothbrush.Apply(toothbrushUpdate(1, 1786192057, 1786207649, nil))

	if toothbrush.active || toothbrush.motion.MotionDetected.Value() {
		t.Fatal("forced stop did not become inactive")
	}
	if toothbrush.lastDuration != 6*time.Second || toothbrush.lastCompleted != 1786207649 {
		t.Fatalf("forced stop: duration=%s completed=%d", toothbrush.lastDuration, toothbrush.lastCompleted)
	}
}

func TestToothbrushWatchdog(t *testing.T) {
	now := time.Unix(1786207645, 0)
	toothbrush := NewToothbrush(config.Device{Name: "T700i"})
	defer toothbrush.Close()
	toothbrush.now = func() time.Time { return now }

	toothbrush.Apply(toothbrushUpdate(0, 1786207643, 1786207645, nil))
	generation := toothbrush.watchdogGeneration
	now = now.Add(toothbrushWatchdog + time.Second)
	toothbrush.expireWatchdog(generation)
	if toothbrush.active || toothbrush.motion.MotionDetected.Value() || toothbrush.lastCompleted != 0 {
		t.Fatalf("watchdog state: active=%t motion=%t completed=%d", toothbrush.active, toothbrush.motion.MotionDetected.Value(), toothbrush.lastCompleted)
	}
}

func TestHistoricalToothbrushEndDoesNotChangeMotion(t *testing.T) {
	now := time.Unix(1786208000, 0)
	toothbrush := NewToothbrush(config.Device{Name: "T700i"})
	defer toothbrush.Close()
	toothbrush.now = func() time.Time { return now }
	score := 88

	toothbrush.Apply(toothbrushUpdate(1, 1786192057, 1786208000, &score))
	if toothbrush.motion.MotionDetected.Value() || toothbrush.active {
		t.Fatal("historical end changed current motion state")
	}
	if toothbrush.lastCompleted != 1786192057 || toothbrush.lastScore == nil || *toothbrush.lastScore != 88 {
		t.Fatalf("historical metadata: completed=%d score=%v", toothbrush.lastCompleted, toothbrush.lastScore)
	}
}

func toothbrushUpdate(eventType int, eventTimestamp, gatewayTimestamp int64, score *int) openmiio.Update {
	return openmiio.Update{
		ProductID:   openmiio.ProductIDToothbrushT700i,
		Kind:        openmiio.ProductKindToothbrush,
		GatewayTime: gatewayTimestamp,
		Toothbrush: &openmiio.ToothbrushEvent{
			Type:      eventType,
			Timestamp: eventTimestamp,
			Score:     score,
		},
	}
}
