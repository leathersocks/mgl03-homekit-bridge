package bridge

import (
	"testing"

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
