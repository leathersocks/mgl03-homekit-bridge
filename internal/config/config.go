package config

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	DefaultBroker                 = "127.0.0.1:1883"
	DefaultTopic                  = "central/report"
	DefaultPort                   = 51826
	DefaultSensorOfflineSeconds   = 15 * 60
	DefaultDiscoveryWindowSeconds = 30

	DiscoveryModeFirst  = "first"
	DiscoveryModeAuto   = "auto"
	DiscoveryModeManual = "manual"
)

type MQTT struct {
	Address  string `json:"address"`
	Topic    string `json:"topic"`
	ClientID string `json:"client_id"`
}

type Device struct {
	Name      string `json:"name"`
	MAC       string `json:"mac,omitempty"`
	DID       string `json:"did,omitempty"`
	ProductID int    `json:"product_id,omitempty"`
	Model     string `json:"model,omitempty"`
	AID       uint64 `json:"aid,omitempty"`
}

type Discovery struct {
	Mode          string `json:"mode,omitempty"`
	WindowSeconds int    `json:"window_seconds,omitempty"`
}

type Config struct {
	BridgeName           string    `json:"bridge_name"`
	Pin                  string    `json:"pin"`
	Port                 int       `json:"port"`
	Interfaces           []string  `json:"interfaces,omitempty"`
	StateDir             string    `json:"state_dir"`
	SensorOfflineSeconds int       `json:"sensor_offline_seconds,omitempty"`
	Discovery            Discovery `json:"discovery,omitempty"`
	MQTT                 MQTT      `json:"mqtt"`
	Devices              []Device  `json:"devices,omitempty"`
}

func Defaults(configPath string) (Config, error) {
	pin, err := generatePin()
	if err != nil {
		return Config{}, err
	}
	stateDir := filepath.Dir(configPath)
	if stateDir == "." || stateDir == "" {
		stateDir = "state"
	}
	return Config{
		BridgeName:           "MGL03 Bluetooth Bridge",
		Pin:                  pin,
		Port:                 DefaultPort,
		StateDir:             stateDir,
		SensorOfflineSeconds: DefaultSensorOfflineSeconds,
		Discovery: Discovery{
			Mode:          DiscoveryModeAuto,
			WindowSeconds: DefaultDiscoveryWindowSeconds,
		},
		MQTT: MQTT{
			Address:  DefaultBroker,
			Topic:    DefaultTopic,
			ClientID: "mgl03-homekit-bridge",
		},
	}, nil
}

// LoadOrCreate loads configPath. If it does not exist, a secure random HomeKit
// pairing PIN is generated and a default configuration is written there.
func LoadOrCreate(configPath string) (cfg Config, created bool, err error) {
	b, err := os.ReadFile(configPath)
	if err == nil {
		if err := json.Unmarshal(b, &cfg); err != nil {
			return Config{}, false, fmt.Errorf("decode config: %w", err)
		}
		if err := cfg.NormalizeAndValidate(configPath); err != nil {
			return Config{}, false, err
		}
		return cfg, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, false, fmt.Errorf("read config: %w", err)
	}

	cfg, err = Defaults(configPath)
	if err != nil {
		return Config{}, false, err
	}
	if err := cfg.NormalizeAndValidate(configPath); err != nil {
		return Config{}, false, err
	}
	if err := writeJSON(configPath, cfg); err != nil {
		return Config{}, false, fmt.Errorf("create config: %w", err)
	}
	return cfg, true, nil
}

func (c *Config) NormalizeAndValidate(configPath string) error {
	if strings.TrimSpace(c.BridgeName) == "" {
		c.BridgeName = "MGL03 Bluetooth Bridge"
	}
	if c.Port == 0 {
		c.Port = DefaultPort
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if strings.TrimSpace(c.StateDir) == "" {
		c.StateDir = filepath.Dir(configPath)
		if c.StateDir == "." || c.StateDir == "" {
			c.StateDir = "state"
		}
	}
	if c.SensorOfflineSeconds == 0 {
		c.SensorOfflineSeconds = DefaultSensorOfflineSeconds
	}
	if c.SensorOfflineSeconds < 60 || c.SensorOfflineSeconds > 24*60*60 {
		return fmt.Errorf("sensor_offline_seconds must be between 60 and 86400")
	}
	c.Discovery.Mode = strings.ToLower(strings.TrimSpace(c.Discovery.Mode))
	if c.Discovery.Mode == "" {
		// Preserve the legacy single-sensor discovery behavior for existing
		// configuration files that predate the discovery section.
		c.Discovery.Mode = DiscoveryModeFirst
	}
	switch c.Discovery.Mode {
	case DiscoveryModeFirst, DiscoveryModeAuto, DiscoveryModeManual:
	default:
		return fmt.Errorf("discovery.mode must be %q, %q, or %q", DiscoveryModeFirst, DiscoveryModeAuto, DiscoveryModeManual)
	}
	if c.Discovery.WindowSeconds == 0 {
		c.Discovery.WindowSeconds = DefaultDiscoveryWindowSeconds
	}
	if c.Discovery.WindowSeconds < 5 || c.Discovery.WindowSeconds > 10*60 {
		return fmt.Errorf("discovery.window_seconds must be between 5 and 600")
	}
	if strings.TrimSpace(c.MQTT.Address) == "" {
		c.MQTT.Address = DefaultBroker
	}
	if strings.TrimSpace(c.MQTT.Topic) == "" {
		c.MQTT.Topic = DefaultTopic
	}
	if strings.TrimSpace(c.MQTT.ClientID) == "" {
		c.MQTT.ClientID = "mgl03-homekit-bridge"
	}

	c.Pin = strings.NewReplacer("-", "", " ", "").Replace(c.Pin)
	if err := validatePin(c.Pin); err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(c.Devices))
	seenAID := make(map[uint64]struct{}, len(c.Devices))
	for i := range c.Devices {
		d := &c.Devices[i]
		d.Name = strings.TrimSpace(d.Name)
		d.MAC = NormalizeMAC(d.MAC)
		d.DID = strings.TrimSpace(d.DID)
		if d.Name == "" {
			d.Name = fmt.Sprintf("Temperature Sensor %d", i+1)
		}
		if d.MAC == "" && d.DID == "" {
			return fmt.Errorf("devices[%d] must have mac or did", i)
		}
		if d.MAC != "" {
			valid, _ := regexp.MatchString(`^([0-9a-f]{2}:){5}[0-9a-f]{2}$`, d.MAC)
			if !valid {
				return fmt.Errorf("devices[%d] has invalid mac %q", i, d.MAC)
			}
		}
		key := d.MAC
		if key == "" {
			key = "did:" + d.DID
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate device %q", key)
		}
		seen[key] = struct{}{}
		if d.AID == 1 {
			return fmt.Errorf("devices[%d] uses reserved bridge aid 1", i)
		}
		if d.AID > 0 {
			if _, ok := seenAID[d.AID]; ok {
				return fmt.Errorf("duplicate device aid %d", d.AID)
			}
			seenAID[d.AID] = struct{}{}
		}
	}
	return nil
}

func NormalizeMAC(s string) string {
	hex := regexp.MustCompile(`[^0-9A-Fa-f]`).ReplaceAllString(s, "")
	if len(hex) != 12 {
		return strings.ToLower(strings.TrimSpace(s))
	}
	parts := make([]string, 6)
	for i := range parts {
		parts[i] = strings.ToLower(hex[i*2 : i*2+2])
	}
	return strings.Join(parts, ":")
}

func FormatPin(pin string) string {
	if len(pin) != 8 {
		return pin
	}
	return pin[:3] + "-" + pin[3:5] + "-" + pin[5:]
}

var invalidPins = map[string]bool{
	"00000000": true, "11111111": true, "22222222": true,
	"33333333": true, "44444444": true, "55555555": true,
	"66666666": true, "77777777": true, "88888888": true,
	"99999999": true, "12345678": true, "87654321": true,
}

func validatePin(pin string) error {
	if ok, _ := regexp.MatchString(`^\d{8}$`, pin); !ok {
		return fmt.Errorf("pin must contain exactly 8 digits")
	}
	if invalidPins[pin] {
		return fmt.Errorf("pin %q is rejected by HomeKit; choose a less predictable value", pin)
	}
	return nil
}

func generatePin() (string, error) {
	for {
		var b [4]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", fmt.Errorf("generate HomeKit PIN: %w", err)
		}
		pin := fmt.Sprintf("%08d", binary.LittleEndian.Uint32(b[:])%100000000)
		if validatePin(pin) == nil {
			return pin, nil
		}
	}
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return err
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
	return os.Rename(tmpName, path)
}
