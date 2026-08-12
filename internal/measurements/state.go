package measurements

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leathersocks/mgl03-homekit-bridge/internal/config"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/openmiio"
)

const currentVersion = 1

// Reading contains only values that are safe to restore after a restart.
// Transient state such as toothbrush motion is deliberately not persisted.
type Reading struct {
	Temperature *float64 `json:"temperature,omitempty"`
	Humidity    *float64 `json:"humidity,omitempty"`
	Battery     *int     `json:"battery,omitempty"`
	UpdatedAt   int64    `json:"updated_at,omitempty"`
}

type file struct {
	Version int                `json:"version"`
	Devices map[string]Reading `json:"devices"`
}

// State stores the last known measurements keyed by normalized MAC address or
// DID. It is owned by the bridge update goroutine and does not require locking.
type State struct {
	devices map[string]Reading
}

func New() *State {
	return &State{devices: make(map[string]Reading)}
}

func Load(path string) (*State, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read measurement cache: %w", err)
	}
	var data file
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("decode measurement cache: %w", err)
	}
	if data.Version != currentVersion {
		return nil, fmt.Errorf("unsupported measurement cache version %d", data.Version)
	}
	state := New()
	for key, reading := range data.Devices {
		state.devices[normalizeKey(key)] = cloneReading(reading)
	}
	return state, nil
}

// Merge records persistent values from update and reports whether the cache
// changed. Identical reports do not cause another flash write.
func (s *State) Merge(update openmiio.Update, now time.Time) bool {
	if s == nil {
		return false
	}
	key := identity(update.MAC, update.DID)
	if key == "" {
		return false
	}
	reading, ok := s.devices[key]
	changed := false

	if !ok && update.DID != "" {
		didKey := "did:" + strings.TrimSpace(update.DID)
		if prior, found := s.devices[didKey]; found {
			reading = prior
			delete(s.devices, didKey)
			changed = true
		}
	}
	if update.Temperature != nil && !sameFloat(reading.Temperature, *update.Temperature) {
		reading.Temperature = floatPtr(*update.Temperature)
		changed = true
	}
	if update.Humidity != nil && !sameFloat(reading.Humidity, *update.Humidity) {
		reading.Humidity = floatPtr(*update.Humidity)
		changed = true
	}
	if update.Battery != nil && (reading.Battery == nil || *reading.Battery != *update.Battery) {
		reading.Battery = intPtr(*update.Battery)
		changed = true
	}
	if !changed {
		return false
	}
	reading.UpdatedAt = update.GatewayTime
	if reading.UpdatedAt <= 0 {
		reading.UpdatedAt = now.Unix()
	}
	s.devices[key] = reading
	return true
}

// Restore returns the persistent values for device. Toothbrush motion is not
// represented in Reading, so every transient accessory starts inactive.
func (s *State) Restore(device config.Device) (openmiio.Update, int64, bool) {
	if s == nil {
		return openmiio.Update{}, 0, false
	}
	key := identity(device.MAC, device.DID)
	reading, ok := s.devices[key]
	if !ok && device.DID != "" {
		reading, ok = s.devices["did:"+strings.TrimSpace(device.DID)]
	}
	if !ok || (reading.Temperature == nil && reading.Humidity == nil && reading.Battery == nil) {
		return openmiio.Update{}, 0, false
	}
	return openmiio.Update{
		MAC:         config.NormalizeMAC(device.MAC),
		DID:         device.DID,
		ProductID:   device.ProductID,
		Model:       device.Model,
		Temperature: cloneFloat(reading.Temperature),
		Humidity:    cloneFloat(reading.Humidity),
		Battery:     cloneInt(reading.Battery),
	}, reading.UpdatedAt, true
}

func (s *State) Save(path string) error {
	if s == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create measurement cache directory: %w", err)
	}
	b, err := json.MarshalIndent(file{Version: currentVersion, Devices: s.devices}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode measurement cache: %w", err)
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".measurements-*.tmp")
	if err != nil {
		return fmt.Errorf("create measurement cache: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("activate measurement cache: %w", err)
	}
	return nil
}

func identity(mac, did string) string {
	if mac = config.NormalizeMAC(mac); mac != "" {
		return mac
	}
	if did = strings.TrimSpace(did); did != "" {
		return "did:" + did
	}
	return ""
}

func normalizeKey(key string) string {
	if strings.HasPrefix(key, "did:") {
		return "did:" + strings.TrimSpace(strings.TrimPrefix(key, "did:"))
	}
	return config.NormalizeMAC(key)
}

func sameFloat(current *float64, next float64) bool {
	return current != nil && *current == next
}

func cloneReading(reading Reading) Reading {
	reading.Temperature = cloneFloat(reading.Temperature)
	reading.Humidity = cloneFloat(reading.Humidity)
	reading.Battery = cloneInt(reading.Battery)
	return reading
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	return floatPtr(*value)
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	return intPtr(*value)
}

func floatPtr(value float64) *float64 { return &value }
func intPtr(value int) *int           { return &value }
