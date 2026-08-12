package bridge

import (
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/service"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/config"
	"github.com/leathersocks/mgl03-homekit-bridge/internal/openmiio"
)

const (
	toothbrushLiveWindow = 60 * time.Second
	toothbrushMaxSession = 10 * time.Minute
	toothbrushWatchdog   = 30 * time.Second
)

type Toothbrush struct {
	Device    config.Device
	Accessory *accessory.A
	motion    *service.MotionSensor
	battery   *service.BatteryService

	mu                 sync.Mutex
	now                func() time.Time
	timer              *time.Timer
	watchdogGeneration uint64
	active             bool
	sessionStart       int64
	lastActivity       time.Time
	lastCompleted      int64
	lastScore          *int
	lastDuration       time.Duration
}

func NewToothbrush(device config.Device) *Toothbrush {
	info := accessoryInfo(device, openmiio.ModelToothbrushT700i, "Xiaomi")
	motion := accessory.NewMotionSensor(info)
	battery := service.NewBatteryService()
	motion.A.AddS(battery.S)
	motion.A.Id = accessoryID(device)
	_ = battery.ChargingState.SetValue(2) // non-rechargeable / not chargeable

	return &Toothbrush{
		Device:    device,
		Accessory: motion.A,
		motion:    motion.MotionSensor,
		battery:   battery,
		now:       time.Now,
	}
}

func (t *Toothbrush) DeviceConfig() config.Device {
	return t.Device
}

func (t *Toothbrush) HAPAccessory() *accessory.A {
	return t.Accessory
}

func (t *Toothbrush) Apply(update openmiio.Update) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if update.Battery != nil {
		applyBattery(t.battery, *update.Battery)
	}
	if update.Toothbrush == nil {
		return
	}

	event := update.Toothbrush
	reference := update.GatewayTime
	if reference <= 0 {
		reference = t.now().Unix()
	}
	rawLive := event.Timestamp > 0 && durationAbs(reference-event.Timestamp) <= toothbrushLiveWindow

	if event.Type == 0 {
		if !rawLive {
			return
		}
		validCurrentStart := t.active && t.sessionStart > 0 && event.Timestamp >= t.sessionStart &&
			time.Duration(event.Timestamp-t.sessionStart)*time.Second <= toothbrushMaxSession
		started := false
		if !validCurrentStart {
			t.sessionStart = event.Timestamp
			if t.sessionStart <= 0 {
				t.sessionStart = reference
			}
			t.active = true
			started = true
		}
		t.lastActivity = t.now()
		t.motion.MotionDetected.SetValue(true)
		t.armWatchdogLocked()
		if started {
			log.Printf("%s brushing started", t.logLabel())
		}
		return
	}

	forcedStop := t.active && t.sessionStart > 0 && !rawLive && reference >= t.sessionStart &&
		time.Duration(reference-t.sessionStart)*time.Second <= toothbrushMaxSession
	effectiveTimestamp := event.Timestamp
	if forcedStop {
		effectiveTimestamp = reference
		log.Printf("%s forced stop inferred: raw_event_ts=%d gwts=%d start_ts=%d", t.logLabel(), event.Timestamp, reference, t.sessionStart)
	}
	live := rawLive || forcedStop

	if live {
		start := t.sessionStart
		wasActive := t.active
		t.stopWatchdogLocked()
		t.motion.MotionDetected.SetValue(false)
		t.active = false
		t.sessionStart = 0
		t.lastActivity = time.Time{}
		if wasActive && start > 0 && effectiveTimestamp >= start {
			duration := time.Duration(effectiveTimestamp-start) * time.Second
			if duration <= toothbrushMaxSession {
				t.lastDuration = duration
				log.Printf("%s brushing completed: duration=%s score=%s forced_stop=%t", t.logLabel(), duration, scoreText(event.Score), forcedStop)
			}
		}
	}

	if effectiveTimestamp > t.lastCompleted {
		t.lastCompleted = effectiveTimestamp
		if event.Score != nil {
			score := *event.Score
			t.lastScore = &score
		}
	}
}

func (t *Toothbrush) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopWatchdogLocked()
}

func (t *Toothbrush) armWatchdogLocked() {
	t.watchdogGeneration++
	generation := t.watchdogGeneration
	if t.timer != nil {
		t.timer.Stop()
	}
	t.timer = time.AfterFunc(toothbrushWatchdog, func() {
		t.expireWatchdog(generation)
	})
}

func (t *Toothbrush) stopWatchdogLocked() {
	t.watchdogGeneration++
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
}

func (t *Toothbrush) expireWatchdog(generation uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if generation != t.watchdogGeneration || !t.active || t.lastActivity.IsZero() {
		return
	}
	elapsed := t.now().Sub(t.lastActivity)
	if elapsed < toothbrushWatchdog {
		t.timer = time.AfterFunc(toothbrushWatchdog-elapsed, func() {
			t.expireWatchdog(generation)
		})
		return
	}
	if t.timer != nil {
		t.timer.Stop()
	}
	t.watchdogGeneration++
	t.timer = nil
	t.active = false
	t.sessionStart = 0
	t.lastActivity = time.Time{}
	t.motion.MotionDetected.SetValue(false)
	log.Printf("%s brushing watchdog timeout after %s; forcing inactive", t.logLabel(), toothbrushWatchdog)
}

func (t *Toothbrush) logLabel() string {
	if t.Device.Name != "" {
		return t.Device.Name
	}
	if t.Device.MAC != "" {
		return t.Device.MAC
	}
	return t.Device.DID
}

func durationAbs(seconds int64) time.Duration {
	if seconds < 0 {
		seconds = -seconds
	}
	return time.Duration(seconds) * time.Second
}

func scoreText(score *int) string {
	if score == nil {
		return "-"
	}
	return strconv.Itoa(*score)
}
