package openmiio

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	ProductIDSensorHTO2 = 5860
	ModelSensorHTO2     = "miaomiaoce.sensor_ht.o2"

	eventTemperature = 19457 // 0x4C01, float32 little-endian
	eventHumidity    = 19458 // 0x4C02, uint8
	eventBattery     = 18435 // 0x4803, uint8
)

type Update struct {
	DID         string
	MAC         string
	ProductID   int
	Model       string
	FrameCount  int
	Temperature *float64
	Humidity    *float64
	Battery     *int
}

func (u Update) HasMeasurements() bool {
	return u.Temperature != nil || u.Humidity != nil || u.Battery != nil
}

type envelope struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// Parse decodes the JSON messages published by openmiio_agent to
// central/report or miio/report. Unrelated devices and methods return an empty
// slice.
func Parse(payload []byte) ([]Update, error) {
	var env envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, fmt.Errorf("decode openmiio envelope: %w", err)
	}
	switch env.Method {
	case "_async.ble_event":
		return parseBLEEvent(env.Params)
	case "properties_changed":
		return parseMIoTProperties(env.Params)
	default:
		return nil, nil
	}
}

func parseBLEEvent(raw json.RawMessage) ([]Update, error) {
	var event struct {
		Dev struct {
			DID  string `json:"did"`
			MAC  string `json:"mac"`
			PDID int    `json:"pdid"`
		} `json:"dev"`
		Events []struct {
			ID   int    `json:"eid"`
			Data string `json:"edata"`
		} `json:"evt"`
		FrameCount int `json:"frmCnt"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return nil, fmt.Errorf("decode BLE event: %w", err)
	}
	if event.Dev.PDID != ProductIDSensorHTO2 {
		return nil, nil
	}

	u := Update{
		DID:        event.Dev.DID,
		MAC:        NormalizeMAC(event.Dev.MAC),
		ProductID:  event.Dev.PDID,
		Model:      ModelSensorHTO2,
		FrameCount: event.FrameCount,
	}
	var decodeErrors []string
	for _, item := range event.Events {
		switch item.ID {
		case eventTemperature:
			value, err := decodeFloat32LE(item.Data)
			if err != nil {
				decodeErrors = append(decodeErrors, "temperature: "+err.Error())
				continue
			}
			value = math.Round(value*10) / 10
			if value < -100 || value > 150 {
				decodeErrors = append(decodeErrors, fmt.Sprintf("temperature out of range: %.2f", value))
				continue
			}
			u.Temperature = &value
		case eventHumidity:
			value, err := decodeByte(item.Data)
			if err != nil {
				decodeErrors = append(decodeErrors, "humidity: "+err.Error())
				continue
			}
			f := float64(value)
			if f > 100 {
				decodeErrors = append(decodeErrors, fmt.Sprintf("humidity out of range: %d", value))
				continue
			}
			u.Humidity = &f
		case eventBattery:
			value, err := decodeByte(item.Data)
			if err != nil {
				decodeErrors = append(decodeErrors, "battery: "+err.Error())
				continue
			}
			if value > 100 {
				decodeErrors = append(decodeErrors, fmt.Sprintf("battery out of range: %d", value))
				continue
			}
			u.Battery = &value
		}
	}
	if len(decodeErrors) > 0 {
		return []Update{u}, fmt.Errorf("decode %s: %s", ModelSensorHTO2, strings.Join(decodeErrors, "; "))
	}
	if !u.HasMeasurements() {
		return nil, nil
	}
	return []Update{u}, nil
}

func parseMIoTProperties(raw json.RawMessage) ([]Update, error) {
	var props []struct {
		DID   string          `json:"did"`
		SIID  int             `json:"siid"`
		PIID  int             `json:"piid"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &props); err != nil {
		return nil, fmt.Errorf("decode MIoT properties: %w", err)
	}

	byDID := make(map[string]*Update)
	order := make([]string, 0)
	for _, prop := range props {
		kind := ""
		switch {
		case prop.SIID == 3 && prop.PIID == 1001:
			kind = "temperature"
		case prop.SIID == 3 && prop.PIID == 1002:
			kind = "humidity"
		case prop.SIID == 2 && prop.PIID == 1003:
			kind = "battery"
		default:
			continue
		}

		u := byDID[prop.DID]
		if u == nil {
			u = &Update{DID: prop.DID, Model: ModelSensorHTO2}
			byDID[prop.DID] = u
			order = append(order, prop.DID)
		}
		value, err := rawNumber(prop.Value)
		if err != nil {
			return nil, fmt.Errorf("decode MIoT %s for %s: %w", kind, prop.DID, err)
		}
		switch kind {
		case "temperature":
			v := math.Round(value*10) / 10
			u.Temperature = &v
		case "humidity":
			v := math.Round(value*10) / 10
			u.Humidity = &v
		case "battery":
			v := int(math.Round(value))
			u.Battery = &v
		}
	}

	updates := make([]Update, 0, len(order))
	for _, did := range order {
		if u := byDID[did]; u.HasMeasurements() {
			updates = append(updates, *u)
		}
	}
	return updates, nil
}

func decodeFloat32LE(s string) (float64, error) {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) < 4 {
		return 0, fmt.Errorf("expected 4-byte little-endian float, got %q", s)
	}
	v := float64(math.Float32frombits(binary.LittleEndian.Uint32(b[:4])))
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("non-finite float %q", s)
	}
	return v, nil
}

func decodeByte(s string) (int, error) {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) < 1 {
		return 0, fmt.Errorf("expected byte, got %q", s)
	}
	return int(b[0]), nil
}

func rawNumber(raw json.RawMessage) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
}

func NormalizeMAC(s string) string {
	hexChars := make([]byte, 0, 12)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			hexChars = append(hexChars, c)
		}
	}
	if len(hexChars) != 12 {
		return strings.ToLower(strings.TrimSpace(s))
	}
	parts := make([]string, 6)
	for i := range parts {
		parts[i] = strings.ToLower(string(hexChars[i*2 : i*2+2]))
	}
	return strings.Join(parts, ":")
}
