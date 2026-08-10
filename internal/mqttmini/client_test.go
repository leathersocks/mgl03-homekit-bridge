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

func TestNormalizedTopics(t *testing.T) {
	got := normalizedTopics([]string{" central/report ", "miio/report", "central/report", ""})
	want := []string{"central/report", "miio/report"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("topics=%q want=%q", got, want)
	}
}

func TestSubscribePayloadIncludesAllTopics(t *testing.T) {
	got := subscribePayload(1, []string{"central/report", "miio/report"})
	want := []byte{
		0, 1,
		0, 14, 'c', 'e', 'n', 't', 'r', 'a', 'l', '/', 'r', 'e', 'p', 'o', 'r', 't', 0,
		0, 11, 'm', 'i', 'i', 'o', '/', 'r', 'e', 'p', 'o', 'r', 't', 0,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("payload=%x want=%x", got, want)
	}
}
