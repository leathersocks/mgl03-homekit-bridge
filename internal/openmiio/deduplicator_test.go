package openmiio

import (
	"testing"
	"time"
)

func TestDeduplicatorHandlesDuplicatesOrderAndWrap(t *testing.T) {
	now := time.Unix(100, 0)
	d := NewDeduplicator(10 * time.Minute)
	d.now = func() time.Time { return now }

	update := Update{MAC: "AA:BB:CC:DD:EE:FF", FrameCount: 254, HasFrameCount: true}
	if !d.Accept(update) || d.Accept(update) {
		t.Fatal("first frame must be accepted and its duplicate rejected")
	}
	update.FrameCount = 253
	if d.Accept(update) {
		t.Fatal("older frame must be rejected")
	}
	update.FrameCount = 255
	if !d.Accept(update) {
		t.Fatal("newer frame must be accepted")
	}
	update.FrameCount = 0
	if !d.Accept(update) {
		t.Fatal("wrapped frame must be accepted")
	}
}

func TestDeduplicatorResetsAfterSilence(t *testing.T) {
	now := time.Unix(100, 0)
	d := NewDeduplicator(time.Minute)
	d.now = func() time.Time { return now }
	first := Update{DID: "blt.test", FrameCount: 100, HasFrameCount: true}
	if !d.Accept(first) {
		t.Fatal("first frame rejected")
	}
	now = now.Add(2 * time.Minute)
	first.FrameCount = 10
	if !d.Accept(first) {
		t.Fatal("counter baseline was not reset")
	}
}

func TestDeduplicatorAllowsMIoTUpdates(t *testing.T) {
	d := NewDeduplicator(time.Minute)
	if !d.Accept(Update{DID: "blt.test"}) {
		t.Fatal("message without frame counter was rejected")
	}
}

func TestDeduplicatorClassifiesDuplicateAndStale(t *testing.T) {
	d := NewDeduplicator(time.Minute)
	update := Update{MAC: "aa:bb:cc:dd:ee:ff", FrameCount: 20, HasFrameCount: true}
	if got := d.Classify(update); got != FrameAccepted {
		t.Fatalf("first disposition = %v", got)
	}
	if got := d.Classify(update); got != FrameDuplicate {
		t.Fatalf("duplicate disposition = %v", got)
	}
	update.FrameCount = 19
	if got := d.Classify(update); got != FrameStale {
		t.Fatalf("stale disposition = %v", got)
	}
}
