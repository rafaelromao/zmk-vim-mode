package state

import (
	"sync"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
)

// Store is the mutable, goroutine-safe holder of everything Decide needs.
// It re-evaluates on every input and calls onChange when the decided code changes.
type Store struct {
	mu       sync.Mutex
	rules    Rules
	clients  map[uint64]*Client
	front    focus.App
	override *Override
	seq      uint64
	now      func() time.Time
	onChange func(Decision)
	last     *Decision
}

// NewStore creates a Store. onChange may be nil.
func NewStore(rules Rules, onChange func(Decision)) *Store {
	return &Store{
		rules:    rules,
		clients:  map[uint64]*Client{},
		now:      time.Now,
		onChange: onChange,
	}
}

// SetClock overrides the clock (tests).
func (s *Store) SetClock(now func() time.Time) {
	s.mu.Lock()
	s.now = now
	s.mu.Unlock()
}

// HelloInfo is what a client announces on connect.
type HelloInfo struct {
	Kind   string
	App    string
	PID    int
	Mode   Mode
	Focus  Focus
	Nested bool
	TTL    time.Duration
}

// Hello registers (or re-registers) a client.
func (s *Store) Hello(id uint64, h HelloInfo) {
	s.mu.Lock()
	s.seq++
	now := s.now()
	c := &Client{ID: id, Kind: h.Kind, App: h.App, PID: h.PID, Mode: h.Mode, Focus: h.Focus,
		Nested: h.Nested, EventSeq: s.seq, LastEvent: now}
	if h.Focus != FocusUnknown {
		c.FocusSeq = s.seq
	}
	if h.TTL > 0 {
		c.Deadline = now.Add(h.TTL)
	}
	s.clients[id] = c
	s.recomputeLocked()
}

// SetMode records a mode report. ttl <= 0 clears any deadline.
func (s *Store) SetMode(id uint64, m Mode, ttl time.Duration) {
	s.mu.Lock()
	c, ok := s.clients[id]
	if !ok {
		s.mu.Unlock()
		return
	}
	s.seq++
	now := s.now()
	c.Mode = m
	c.EventSeq = s.seq
	c.LastEvent = now
	if ttl > 0 {
		c.Deadline = now.Add(ttl)
	} else {
		c.Deadline = time.Time{}
	}
	s.recomputeLocked()
}

// SetFocus records a focus report; mode (if non-nil) is applied atomically.
func (s *Store) SetFocus(id uint64, f Focus, mode *Mode) {
	s.mu.Lock()
	c, ok := s.clients[id]
	if !ok {
		s.mu.Unlock()
		return
	}
	s.seq++
	c.Focus = f
	c.FocusSeq = s.seq
	c.EventSeq = s.seq
	c.LastEvent = s.now()
	if mode != nil {
		c.Mode = *mode
	}
	s.recomputeLocked()
}

// Gone removes a client (connection closed or bye).
func (s *Store) Gone(id uint64) {
	s.mu.Lock()
	if _, ok := s.clients[id]; !ok {
		s.mu.Unlock()
		return
	}
	delete(s.clients, id)
	s.recomputeLocked()
}

// SetFrontmost records the frontmost app. A non-sticky override is cleared
// when the app identity changes.
func (s *Store) SetFrontmost(a focus.App) {
	s.mu.Lock()
	changed := !s.front.SameIdentity(a)
	s.front = a
	if changed && s.override != nil && !s.override.Sticky {
		s.override = nil
	}
	s.recomputeLocked()
}

// SetOverride installs a manual override; nil returns to automatic mode.
// Re-issuing an override with the same mode toggles back to automatic so a
// single hotkey can flip it.
func (s *Store) SetOverride(o *Override) (nowActive bool) {
	s.mu.Lock()
	if o != nil && s.override.Active(s.now()) && s.override.Mode == o.Mode {
		s.override = nil
	} else {
		s.override = o
	}
	active := s.override != nil
	s.recomputeLocked()
	return active
}

// Recompute re-evaluates without new input (TTL / override expiry timers).
func (s *Store) Recompute() {
	s.mu.Lock()
	s.recomputeLocked()
}

// Snapshot returns a copy of the current inputs.
func (s *Store) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

func (s *Store) snapshotLocked() Snapshot {
	cs := make([]Client, 0, len(s.clients))
	for _, c := range s.clients {
		cs = append(cs, *c)
	}
	var o *Override
	if s.override != nil {
		cp := *s.override
		o = &cp
	}
	return Snapshot{Frontmost: s.front, Clients: cs, Override: o, Now: s.now()}
}

// Decision returns the current decision (computing it if needed).
func (s *Store) Decision() Decision {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil {
		d := Decide(s.rules, s.snapshotLocked())
		s.last = &d
	}
	return *s.last
}

// NextDeadline returns the earliest future instant at which the decision may
// change on its own (client TTL or override expiry), if any.
func (s *Store) NextDeadline() (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	var best time.Time
	consider := func(t time.Time) {
		if t.IsZero() || !t.After(now) {
			return
		}
		if best.IsZero() || t.Before(best) {
			best = t
		}
	}
	for _, c := range s.clients {
		consider(c.Deadline)
	}
	if s.override != nil {
		consider(s.override.Until)
	}
	return best, !best.IsZero()
}

// recomputeLocked must be called with s.mu held; it releases the lock.
func (s *Store) recomputeLocked() {
	d := Decide(s.rules, s.snapshotLocked())
	changed := s.last == nil || s.last.Code != d.Code || s.last.Mode != d.Mode
	s.last = &d
	cb := s.onChange
	s.mu.Unlock()
	if changed && cb != nil {
		cb(d)
	}
}
