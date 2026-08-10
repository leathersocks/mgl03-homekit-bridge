package config

import (
	"path/filepath"
	"testing"
)

func TestNormalizeAndValidate(t *testing.T) {
	cfg := Config{Pin: "195-50-224", Devices: []Device{{MAC: "AA-BB-CC-DD-EE-FF"}}}
	if err := cfg.NormalizeAndValidate(filepath.Join(t.TempDir(), "config.json")); err != nil {
		t.Fatal(err)
	}
	if cfg.Pin != "19550224" {
		t.Fatalf("pin = %q", cfg.Pin)
	}
	if cfg.Devices[0].MAC != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("mac = %q", cfg.Devices[0].MAC)
	}
	if cfg.MQTT.Topic != DefaultTopic || cfg.Port != DefaultPort {
		t.Fatalf("defaults were not applied: %#v", cfg)
	}
}

func TestRejectInvalidPin(t *testing.T) {
	cfg := Config{Pin: "12345678"}
	if err := cfg.NormalizeAndValidate("config.json"); err == nil {
		t.Fatal("expected invalid PIN error")
	}
}

func TestRejectInvalidMAC(t *testing.T) {
	cfg := Config{Pin: "19550224", Devices: []Device{{MAC: "not-a-mac"}}}
	if err := cfg.NormalizeAndValidate("config.json"); err == nil {
		t.Fatal("expected invalid MAC error")
	}
}
