package measurements

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leathersocks/mgl03-homekit-bridge/internal/config"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/openmiio"
)

func TestSaveLoadAndRestore(t *testing.T) {
	state := New()
	temperature := 23.4
	humidity := 46.0
	battery := 100
	update := openmiio.Update{
		MAC:         "A4C1386A689F",
		DID:         "blt.1.sensor",
		GatewayTime: 1786541187,
		Temperature: &temperature,
		Humidity:    &humidity,
		Battery:     &battery,
	}
	if !state.Merge(update, time.Unix(1, 0)) {
		t.Fatal("first measurement did not change the cache")
	}
	if state.Merge(update, time.Unix(2, 0)) {
		t.Fatal("identical measurement caused another cache write")
	}

	path := filepath.Join(t.TempDir(), "measurements.json")
	if err := state.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	restored, updatedAt, ok := loaded.Restore(config.Device{
		MAC: "a4:c1:38:6a:68:9f",
		DID: update.DID,
	})
	if !ok || updatedAt != update.GatewayTime {
		t.Fatalf("ok=%t updatedAt=%d", ok, updatedAt)
	}
	if restored.Temperature == nil || *restored.Temperature != temperature ||
		restored.Humidity == nil || *restored.Humidity != humidity ||
		restored.Battery == nil || *restored.Battery != battery {
		t.Fatalf("restored = %#v", restored)
	}
}

func TestMotionIsNotPersisted(t *testing.T) {
	state := New()
	event := &openmiio.ToothbrushEvent{Type: 0, Timestamp: 1786541187}
	if state.Merge(openmiio.Update{MAC: "c0:b5:86:1f:e4:5f", Toothbrush: event}, time.Now()) {
		t.Fatal("transient toothbrush motion was persisted")
	}
	if _, _, ok := state.Restore(config.Device{MAC: "c0:b5:86:1f:e4:5f"}); ok {
		t.Fatal("transient toothbrush state was restored")
	}
}

func TestBatteryOnlyRestore(t *testing.T) {
	state := New()
	battery := 75
	if !state.Merge(openmiio.Update{
		DID:     "blt.1.toothbrush",
		Battery: &battery,
	}, time.Unix(99, 0)) {
		t.Fatal("battery update was not cached")
	}
	restored, updatedAt, ok := state.Restore(config.Device{DID: "blt.1.toothbrush"})
	if !ok || updatedAt != 99 || restored.Battery == nil || *restored.Battery != battery {
		t.Fatalf("restored=%#v updatedAt=%d ok=%t", restored, updatedAt, ok)
	}
	if restored.Toothbrush != nil {
		t.Fatal("toothbrush motion was restored")
	}
}

func TestLoadMissingAndRejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "measurements.json")
	state, err := Load(path)
	if err != nil || state == nil {
		t.Fatalf("missing cache: state=%v err=%v", state, err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("corrupt cache was accepted")
	}
}
