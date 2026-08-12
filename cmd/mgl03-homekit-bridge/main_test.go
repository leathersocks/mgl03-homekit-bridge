package main

import (
	"context"
	"testing"
	"time"

	"github.com/brutella/hap/accessory"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/bridge"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/config"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/measurements"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/openmiio"
)

func TestMergeDevicesMatchesMACOrDID(t *testing.T) {
	configured := []config.Device{{Name: "configured", DID: "blt.3.same"}}
	saved := []config.Device{
		{Name: "duplicate", MAC: "aa:bb:cc:dd:ee:ff", DID: "blt.3.same"},
		{Name: "other", MAC: "00:11:22:33:44:55", DID: "blt.3.other"},
	}
	merged := mergeDevices(configured, saved)
	if len(merged) != 2 || merged[0].Name != "configured" || merged[1].Name != "other" {
		t.Fatalf("merged = %#v", merged)
	}
}

type recordingAccessory struct {
	device  config.Device
	applied chan openmiio.Update
}

func (r *recordingAccessory) DeviceConfig() config.Device  { return r.device }
func (r *recordingAccessory) HAPAccessory() *accessory.A   { return nil }
func (r *recordingAccessory) Apply(update openmiio.Update) { r.applied <- update }
func (r *recordingAccessory) Close()                       {}

func TestProcessUpdatesRefreshesOnlyDuplicateToothbrushStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan openmiio.Update, 3)
	recorder := &recordingAccessory{
		device:  config.Device{MAC: "c0:b5:86:1f:e4:5f"},
		applied: make(chan openmiio.Update, 3),
	}
	var deviceAccessory bridge.DeviceAccessory = recorder
	go processUpdates(
		ctx,
		updates,
		[]bridge.DeviceAccessory{deviceAccessory},
		openmiio.NewDeduplicator(time.Minute),
		nil,
		config.Config{Discovery: config.Discovery{Mode: config.DiscoveryModeManual}},
		"",
		[]config.Device{recorder.device},
		nil,
		"",
	)

	start := openmiio.Update{
		MAC:           recorder.device.MAC,
		FrameCount:    20,
		HasFrameCount: true,
		Toothbrush:    &openmiio.ToothbrushEvent{Type: 0, Timestamp: 100},
	}
	updates <- start
	updates <- start // exact duplicate start refreshes the active watchdog
	stale := start
	stale.FrameCount = 19
	updates <- stale // an older start must not be applied

	for i := 0; i < 2; i++ {
		select {
		case <-recorder.applied:
		case <-time.After(time.Second):
			t.Fatalf("application %d was not delivered", i+1)
		}
	}
	select {
	case update := <-recorder.applied:
		t.Fatalf("stale update was applied: %#v", update)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRestoreCachedMeasurements(t *testing.T) {
	temperature := 23.4
	humidity := 46.0
	state := measurements.New()
	device := config.Device{MAC: "a4:c1:38:6a:68:9f", DID: "blt.1.sensor"}
	if !state.Merge(openmiio.Update{
		MAC:         device.MAC,
		DID:         device.DID,
		Temperature: &temperature,
		Humidity:    &humidity,
	}, time.Unix(1786541187, 0)) {
		t.Fatal("measurement was not cached")
	}
	recorder := &recordingAccessory{device: device, applied: make(chan openmiio.Update, 1)}
	if restored := restoreCachedMeasurements([]bridge.DeviceAccessory{recorder}, state); restored != 1 {
		t.Fatalf("restored = %d", restored)
	}
	select {
	case update := <-recorder.applied:
		if update.Temperature == nil || *update.Temperature != temperature ||
			update.Humidity == nil || *update.Humidity != humidity {
			t.Fatalf("update = %#v", update)
		}
	case <-time.After(time.Second):
		t.Fatal("cached update was not applied")
	}
}

func TestPrepareDevicesResolvesToothbrushModel(t *testing.T) {
	devices := []config.Device{{
		Name:  "T700i",
		MAC:   "c0:b5:86:1f:e4:5f",
		Model: openmiio.ModelToothbrushT700i,
	}}
	prepared, changed := prepareDevices(devices)
	if !changed || prepared[0].ProductID != openmiio.ProductIDToothbrushT700i || prepared[0].AID < 2 {
		t.Fatalf("prepared = %#v changed=%t", prepared, changed)
	}
}

func TestPrepareDevicesPreservesLegacyAIDAndAvoidsCollision(t *testing.T) {
	devices := []config.Device{
		{Name: "one", MAC: "aa:bb:cc:dd:ee:ff"},
		{Name: "two", MAC: "aa:bb:cc:dd:ee:ff", DID: "different"},
	}
	prepared, changed := prepareDevices(devices)
	if !changed {
		t.Fatal("legacy devices were not migrated")
	}
	if prepared[0].AID < 2 || prepared[1].AID < 2 || prepared[0].AID == prepared[1].AID {
		t.Fatalf("aids = %d, %d", prepared[0].AID, prepared[1].AID)
	}
	if prepared[0].ProductID == 0 || prepared[0].Model == "" {
		t.Fatalf("device defaults missing: %#v", prepared[0])
	}
}
