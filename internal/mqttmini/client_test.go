package mqttmini

import (
	"bytes"
	"reflect"
	"testing"
)

func TestRemainingLengthRoundTrip(t *testing.T) {
	for _, n := range []int{0, 1, 127, 128, 16384, maxPacketSize} {
		encoded := encodeRemainingLength(n)
		got, err := decodeRemainingLength(bytes.NewReader(encoded))
		if err != nil || got != n {
			t.Fatalf("n=%d encoded=%v got=%d err=%v", n, encoded, got, err)
		}
	}
}

func TestParsePublish(t *testing.T) {
	payload := append([]byte{0, 14}, []byte("central/report")...)
	payload = append(payload, []byte(`{"ok":true}`)...)
	topic, message, _, qos, err := parsePublish(0x30, payload)
	if err != nil {
		t.Fatal(err)
	}
	if topic != "central/report" || qos != 0 || !reflect.DeepEqual(message, []byte(`{"ok":true}`)) {
		t.Fatalf("topic=%q qos=%d message=%s", topic, qos, message)
	}
}
