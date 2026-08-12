package openmiio

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type BLEEvent struct {
	ID   int    `json:"eid"`
	Data string `json:"edata"`
}

type ToothbrushEvent struct {
	Type      int
	Timestamp int64
	Score     *int
}

type Update struct {
	DID           string
	MAC           string
	ProductID     int
	Kind          ProductKind
	Model         string
	FrameCount    int
	HasFrameCount bool
	GatewayTime   int64
	Temperature   *float64
	Humidity      *float64
	Battery       *int
	Toothbrush    *ToothbrushEvent
}

func (u Update) HasMeasurements() bool {
	return u.Temperature != nil || u.Humidity != nil || u.Battery != nil || u.Toothbrush != nil
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
		Events      []BLEEvent `json:"evt"`
		FrameCount  int        `json:"frmCnt"`
		GatewayTime int64      `json:"gwts"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return nil, fmt.Errorf("decode BLE event: %w", err)
	}
	product, ok := LookupProduct(event.Dev.PDID)
	if !ok {
		return nil, nil
	}

	u := Update{
		DID:           event.Dev.DID,
		MAC:           NormalizeMAC(event.Dev.MAC),
		ProductID:     event.Dev.PDID,
		Kind:          product.Kind,
		Model:         product.Model,
		FrameCount:    event.FrameCount,
		HasFrameCount: true,
		GatewayTime:   event.GatewayTime,
	}
	if product.decode == nil {
		return nil, nil
	}
	decodeErrors := product.decode(event.Events, &u)
	if len(decodeErrors) > 0 {
		return []Update{u}, fmt.Errorf("decode %s: %s", product.Model, strings.Join(decodeErrors, "; "))
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
			if v < -100 || v > 150 {
				return nil, fmt.Errorf("decode MIoT temperature for %s: out of range %.1f", prop.DID, v)
			}
			u.Temperature = &v
		case "humidity":
			v := math.Round(value*10) / 10
			if v < 0 || v > 100 {
				return nil, fmt.Errorf("decode MIoT humidity for %s: out of range %.1f", prop.DID, v)
			}
			u.Humidity = &v
		case "battery":
			v := int(math.Round(value))
			if v < 0 || v > 100 {
				return nil, fmt.Errorf("decode MIoT battery for %s: out of range %d", prop.DID, v)
			}
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
