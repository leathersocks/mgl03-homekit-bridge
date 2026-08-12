package openmiio

import (
	"strings"
	"time"
)

// Deduplicator rejects duplicate and clearly out-of-order BLE frame counters.
// Xiaomi BLE counters are one byte and wrap from 255 to 0. A device is allowed
// to establish a new baseline after ResetAfter so battery replacement and
// gateway restarts do not permanently suppress its reports.
type Deduplicator struct {
	ResetAfter time.Duration
	now        func() time.Time
	frames     map[string]frameState
}

type frameState struct {
	counter uint8
	seenAt  time.Time
}

func NewDeduplicator(resetAfter time.Duration) *Deduplicator {
	return &Deduplicator{
		ResetAfter: resetAfter,
		now:        time.Now,
		frames:     make(map[string]frameState),
	}
}

// Accept returns true for messages without a BLE frame counter and for the
// first/newer frame of each device. The device identity prefers MAC over DID.
func (d *Deduplicator) Accept(update Update) bool {
	if !update.HasFrameCount {
		return true
	}
	key := strings.ToLower(update.MAC)
	if key == "" {
		key = "did:" + update.DID
	}
	if key == "did:" {
		return true
	}

	now := d.now()
	next := uint8(update.FrameCount)
	previous, ok := d.frames[key]
	if !ok || d.ResetAfter <= 0 || now.Sub(previous.seenAt) >= d.ResetAfter {
		d.frames[key] = frameState{counter: next, seenAt: now}
		return true
	}

	delta := uint8(next - previous.counter)
	if delta == 0 || delta > 127 {
		return false
	}
	d.frames[key] = frameState{counter: next, seenAt: now}
	return true
}
