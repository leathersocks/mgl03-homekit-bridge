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
