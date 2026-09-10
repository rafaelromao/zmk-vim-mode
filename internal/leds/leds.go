// Package leds abstracts "write a 3-bit code to every matching keyboard" over
// platform backends and implements the reconciler that keeps the keyboards in
// the desired state across hotplug, clobbers and daemon transitions.
package leds

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/proto"
	"github.com/rafaelromao/zmk-vim-mode/internal/state"
)

// DeviceID is a backend-specific stable identifier for a keyboard.
type DeviceID string

// Device describes one keyboard.
type Device struct {
	ID        DeviceID
	VID, PID  uint16
	Product   string
	Transport string // usb | bluetooth | unknown
	Writable  bool
	Note      string
}

// EventKind enumerates backend events.
type EventKind int

const (
	Added   EventKind = iota // device appeared (also emitted for devices present at start)
	Removed                  // device disappeared
	Echo                     // the OS rewrote the device's LEDs; our bits may be clobbered
	Wake                     // system woke from sleep; re-assert everything
)

// Event is a backend notification.
type Event struct {
	Kind EventKind
	Dev  DeviceID
}

// Backend is a platform LED writer.
type Backend interface {
	// Start scans devices, emits Added for each, then watches for hotplug/echo
	// until ctx is done. It must not block the caller.
	Start(ctx context.Context, events chan<- Event) error
	Devices() []Device
	// Write sends the 3-bit code (b0 Compose, b1 Kana, b2 Scroll Lock) to dev.
	Write(dev DeviceID, code uint8) error
}

// Reconciler owns the desired code and drives writes.
type Reconciler struct {
	be  Backend
	log *slog.Logger

	Coalesce      time.Duration // SetDesired debounce (latest wins)
	EchoMinGap    time.Duration // minimum gap between echo-triggered re-asserts per device
	RetryDelays   []time.Duration
	WriteWatchdog time.Duration

	mu           sync.Mutex
	desired      uint8
	hasDesired   bool
	last         map[DeviceID]uint8
	lastReassert map[DeviceID]time.Time
	pendingEcho  map[DeviceID]*time.Timer
	setTimer     *time.Timer
	pendingSet   uint8
	writes       int
}

// NewReconciler creates a Reconciler with plan defaults (10 ms coalesce,
// 10 ms echo gap, +50/+300/+1000 ms retries).
func NewReconciler(be Backend, log *slog.Logger) *Reconciler {
	if log == nil {
		log = slog.Default()
	}
	return &Reconciler{
		be: be, log: log,
		Coalesce:      10 * time.Millisecond,
		EchoMinGap:    10 * time.Millisecond,
		RetryDelays:   []time.Duration{50 * time.Millisecond, 300 * time.Millisecond, time.Second},
		WriteWatchdog: 200 * time.Millisecond,
		last:          map[DeviceID]uint8{},
		lastReassert:  map[DeviceID]time.Time{},
		pendingEcho:   map[DeviceID]*time.Timer{},
	}
}

// Desired returns the current desired code.
func (r *Reconciler) Desired() (uint8, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.desired, r.hasDesired
}

// SetDesired records a state transition. Bursts within Coalesce collapse to
// the latest value; the write is a *transition* (plain code, so Legacy runs
// its one-time binding).
func (r *Reconciler) SetDesired(code uint8) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pendingSet = code
	if r.setTimer != nil {
		r.setTimer.Stop()
	}
	if r.Coalesce <= 0 {
		r.applyDesiredLocked()
		return
	}
	r.setTimer = time.AfterFunc(r.Coalesce, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.applyDesiredLocked()
	})
}

func (r *Reconciler) applyDesiredLocked() {
	code := r.pendingSet
	r.desired, r.hasDesired = code, true
	for _, d := range r.be.Devices() {
		r.writeLocked(d.ID, code, false, "transition")
	}
}

// Reassert re-writes the desired code to one device using the silent alias
// (Legacy → LegacySilent). force bypasses the per-device dedupe (used after an
// echo showed the OS overwrote our bits).
func (r *Reconciler) Reassert(dev DeviceID, force bool, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.hasDesired {
		return
	}
	r.writeLocked(dev, state.SilentAlias(r.desired), force, reason)
}

// WriteAll writes code to every device immediately (shutdown: 0).
func (r *Reconciler) WriteAll(code uint8) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range r.be.Devices() {
		r.writeLocked(d.ID, code, true, "write-all")
	}
}

// writeLocked performs a deduped write with retries on error.
func (r *Reconciler) writeLocked(dev DeviceID, code uint8, force bool, reason string) {
	if !force {
		if last, ok := r.last[dev]; ok {
			if last == code {
				return
			}
			// A device already showing the silent alias is in the equivalent state.
			if code == state.CodeLegacy && last == state.CodeLegacySilent {
				return
			}
		}
	}
	r.doWrite(dev, code, reason, 0)
}

func (r *Reconciler) doWrite(dev DeviceID, code uint8, reason string, attempt int) {
	start := time.Now()
	err := r.be.Write(dev, code)
	if d := time.Since(start); d > r.WriteWatchdog && r.WriteWatchdog > 0 {
		r.log.Warn("slow LED write", "dev", dev, "took", d)
	}
	if err == nil {
		r.last[dev] = code
		r.writes++
		// Info, not debug: one line per decision change is cheap, and "the
		// daemon decided right but the keyboard did not move" is undiagnosable
		// without it.
		r.log.Info("led write", "dev", dev, "code", code, "reason", reason)
		return
	}
	if attempt >= len(r.RetryDelays) {
		r.log.Warn("LED write failed, giving up", "dev", dev, "code", code, "err", err)
		return
	}
	delay := r.RetryDelays[attempt]
	r.log.Debug("LED write failed, retrying", "dev", dev, "code", code, "err", err, "in", delay)
	time.AfterFunc(delay, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if !r.hasDesired {
			return
		}
		// Re-derive the code at retry time: desired may have moved on.
		want := code
		if code != r.desired && code != state.SilentAlias(r.desired) {
			want = state.SilentAlias(r.desired)
		}
		r.doWrite(dev, want, reason+" (retry)", attempt+1)
	})
}

// Run consumes backend events until ctx is done.
func (r *Reconciler) Run(ctx context.Context, events <-chan Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			r.handle(ev)
		}
	}
}

func (r *Reconciler) handle(ev Event) {
	switch ev.Kind {
	case Added:
		r.mu.Lock()
		delete(r.last, ev.Dev)
		r.mu.Unlock()
		r.Reassert(ev.Dev, true, "device added")
	case Removed:
		r.mu.Lock()
		delete(r.last, ev.Dev)
		delete(r.lastReassert, ev.Dev)
		if t := r.pendingEcho[ev.Dev]; t != nil {
			t.Stop()
			delete(r.pendingEcho, ev.Dev)
		}
		r.mu.Unlock()
	case Wake:
		r.mu.Lock()
		devs := r.be.Devices()
		r.mu.Unlock()
		for _, d := range devs {
			r.Reassert(d.ID, true, "wake")
		}
	case Echo:
		r.echo(ev.Dev)
	}
}

// echo schedules a forced re-assert, rate-limited per device.
func (r *Reconciler) echo(dev DeviceID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.hasDesired {
		return
	}
	if r.pendingEcho[dev] != nil {
		return // already scheduled
	}
	wait := r.EchoMinGap - time.Since(r.lastReassert[dev])
	fire := func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.pendingEcho, dev)
		r.lastReassert[dev] = time.Now()
		r.writeLocked(dev, state.SilentAlias(r.desired), true, "echo")
	}
	if wait <= 0 {
		r.lastReassert[dev] = time.Now()
		r.writeLocked(dev, state.SilentAlias(r.desired), true, "echo")
		return
	}
	r.pendingEcho[dev] = time.AfterFunc(wait, fire)
}

// Devices returns the backend devices decorated with the last written code.
func (r *Reconciler) Devices() []proto.Device {
	r.mu.Lock()
	defer r.mu.Unlock()
	devs := r.be.Devices()
	out := make([]proto.Device, 0, len(devs))
	for _, d := range devs {
		pd := proto.Device{ID: string(d.ID), Product: d.Product, Transport: d.Transport, VID: d.VID, PID: d.PID, Writable: d.Writable, Note: d.Note}
		if c, ok := r.last[d.ID]; ok {
			pd.LastCode = proto.U8(c)
		}
		out = append(out, pd)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Writes returns the number of successful writes (tests/status).
func (r *Reconciler) Writes() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writes
}
