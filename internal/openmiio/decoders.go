package openmiio

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
)

const (
	eventTemperature = 19457 // 0x4C01, float32 little-endian
	eventHumidity    = 19458 // 0x4C02, uint8
	eventBattery     = 18435 // 0x4803, uint8

	eventToothbrushState = 12291 // 0x3003, type + Unix timestamp + optional score
	eventStandardBattery = 4106  // 0x100A, uint8
)

func decodeSensorHTO2(events []BLEEvent, update *Update) []string {
	var decodeErrors []string
	for _, item := range events {
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
			update.Temperature = &value
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
			update.Humidity = &f
		case eventBattery:
			decodeBattery(item.Data, update, &decodeErrors)
		}
	}
	return decodeErrors
}

func decodeToothbrushT700i(events []BLEEvent, update *Update) []string {
	var decodeErrors []string
	for _, item := range events {
		switch item.ID {
		case eventToothbrushState:
			data, err := hex.DecodeString(item.Data)
			if err != nil || len(data) < 5 {
				decodeErrors = append(decodeErrors, fmt.Sprintf("toothbrush: expected type and 4-byte timestamp, got %q", item.Data))
				continue
			}
			event := &ToothbrushEvent{
				Type:      int(data[0]),
				Timestamp: int64(binary.LittleEndian.Uint32(data[1:5])),
			}
			if len(data) >= 6 {
				score := int(data[5])
				event.Score = &score
			}
			update.Toothbrush = event
		case eventStandardBattery:
			decodeBattery(item.Data, update, &decodeErrors)
		}
	}
	return decodeErrors
}

func decodeBattery(data string, update *Update, decodeErrors *[]string) {
	value, err := decodeByte(data)
	if err != nil {
		*decodeErrors = append(*decodeErrors, "battery: "+err.Error())
		return
	}
	if value > 100 {
		*decodeErrors = append(*decodeErrors, fmt.Sprintf("battery out of range: %d", value))
		return
	}
	update.Battery = &value
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
