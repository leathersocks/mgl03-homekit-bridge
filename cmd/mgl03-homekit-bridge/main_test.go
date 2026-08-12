package main

import (
	"testing"

	"github.com/leathersocks/mgl03-homekit-bridge/internal/config"
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
