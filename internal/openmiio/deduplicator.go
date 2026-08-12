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

type FrameDisposition uint8

const (
	FrameAccepted FrameDisposition = iota
	FrameDuplicate
	FrameStale
)

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
	return d.Classify(update) == FrameAccepted
}

// Classify distinguishes a duplicate from an older frame. Most products drop
// both, while an active toothbrush may use a duplicate start advertisement to
// refresh its lost-end watchdog without treating an old frame as a new start.
func (d *Deduplicator) Classify(update Update) FrameDisposition {
	if !update.HasFrameCount {
		return FrameAccepted
	}
	key := strings.ToLower(update.MAC)
	if key == "" {
		key = "did:" + update.DID
	}
	if key == "did:" {
		return FrameAccepted
	}

	now := d.now()
	next := uint8(update.FrameCount)
	previous, ok := d.frames[key]
	if !ok || d.ResetAfter <= 0 || now.Sub(previous.seenAt) >= d.ResetAfter {
		d.frames[key] = frameState{counter: next, seenAt: now}
		return FrameAccepted
	}

	delta := uint8(next - previous.counter)
	if delta == 0 {
		return FrameDuplicate
	}
	if delta > 127 {
		return FrameStale
	}
	d.frames[key] = frameState{counter: next, seenAt: now}
	return FrameAccepted
}
