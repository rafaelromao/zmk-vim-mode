// Package daemon wires the pieces together: unix-socket server → state store
// → reconciler → LED backend, plus the focus watcher and the CLI request
// handlers (set/status/devices).
package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
	"github.com/rafaelromao/zmk-vim-mode/internal/leds"
	"github.com/rafaelromao/zmk-vim-mode/internal/proto"
	"github.com/rafaelromao/zmk-vim-mode/internal/server"
	"github.com/rafaelromao/zmk-vim-mode/internal/state"
)

// Options configures a Daemon.
type Options struct {
	SocketPath string
	Rules      state.Rules
	Focus      focus.Watcher
	Backend    leds.Backend
	Log        *slog.Logger
	Version    string
	// StartupOffDelay delays the first OFF write after start so clients of a
	// restarted daemon can reconnect without a visible OFF flicker.
	StartupOffDelay time.Duration
	// TickEvery drives TTL/override expiry re-evaluation.
	TickEvery time.Duration
}

// Daemon is the running service.
type Daemon struct {
	o     Options
	log   *slog.Logger
	store *state.Store
	rec   *leds.Reconciler
	srv   *server.Server

	mu        sync.Mutex
	clients   map[server.ConnID]bool // connections that sent hello
	started   time.Time
	gateUntil time.Time
	gateTimer *time.Timer
}

// New creates a Daemon. Backend and Focus are required.
func New(o Options) (*Daemon, error) {
	if o.Backend == nil || o.Focus == nil {
		return nil, fmt.Errorf("daemon: Backend and Focus are required")
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.SocketPath == "" {
		o.SocketPath = server.DefaultSocketPath()
	}
	if o.StartupOffDelay == 0 {
		o.StartupOffDelay = 1500 * time.Millisecond
	}
	if o.TickEvery == 0 {
		o.TickEvery = 200 * time.Millisecond
	}
	if o.Version == "" {
		o.Version = "dev"
	}
	d := &Daemon{o: o, log: o.Log, clients: map[server.ConnID]bool{}}
	d.store = state.NewStore(o.Rules, d.onDecision)
	d.rec = leds.NewReconciler(o.Backend, o.Log)
	return d, nil
}

// Run serves until ctx is done, then turns the keyboards OFF.
func (d *Daemon) Run(ctx context.Context) error {
	srv, err := server.Listen(d.o.SocketPath, d, d.log)
	if err != nil {
		return err
	}
	d.srv = srv
	d.started = time.Now()
	d.gateUntil = d.started.Add(d.o.StartupOffDelay)
	d.log.Info("listening", "socket", d.o.SocketPath, "version", d.o.Version)

	events := make(chan leds.Event, 64)
	if err := d.o.Backend.Start(ctx, events); err != nil {
		srv.Close()
		return fmt.Errorf("start LED backend: %w", err)
	}
	go d.rec.Run(ctx, events)
	go func() {
		// Log every frontmost change: "frontmost unknown" is otherwise
		// indistinguishable between a watcher that has not reported yet and one
		// that cannot find the compositor. Title changes of the same window
		// arrive too (VSCode publishes its focused view there) and stay at
		// debug level, as do titles in general.
		var last focus.App
		emit := func(a focus.App) {
			switch {
			case !a.Known:
				d.log.Info("frontmost unknown (focus backend unavailable); trusting editor clients")
			case !last.SameIdentity(a):
				d.log.Info("frontmost", "class", a.Class, "pid", a.PID)
				d.log.Debug("frontmost title", "title", a.Title)
			default:
				d.log.Debug("frontmost title", "title", a.Title)
			}
			last = a
			d.store.SetFrontmost(a)
		}
		if err := d.o.Focus.Run(ctx, emit); err != nil && ctx.Err() == nil {
			d.log.Warn("focus watcher stopped", "err", err)
			d.store.SetFrontmost(focus.App{}) // unknown → fail open
		}
	}()
	go d.tick(ctx)

	// Apply the initial decision (usually OFF, gated).
	d.onDecision(d.store.Decision())

	err = srv.Serve(ctx)
	// Shutdown: never leave the keyboard stuck in a vim layer.
	d.rec.WriteAll(state.CodeOff)
	return err
}

func (d *Daemon) tick(ctx context.Context) {
	t := time.NewTicker(d.o.TickEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, ok := d.store.NextDeadline(); ok {
				continue // nothing expired yet
			}
			d.store.Recompute()
		}
	}
}

// onDecision is the store's change callback.
func (d *Daemon) onDecision(dec state.Decision) {
	d.log.Info("decision", "mode", dec.Mode.String(), "code", dec.Code, "reason", dec.Reason, "client", dec.ClientID)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.gateTimer != nil {
		d.gateTimer.Stop()
		d.gateTimer = nil
	}
	if dec.Code == state.CodeOff && time.Now().Before(d.gateUntil) {
		wait := time.Until(d.gateUntil)
		d.log.Debug("gating initial OFF", "for", wait)
		d.gateTimer = time.AfterFunc(wait, func() {
			if cur := d.store.Decision(); cur.Code == state.CodeOff {
				d.rec.SetDesired(state.CodeOff)
			}
		})
		return
	}
	d.rec.SetDesired(dec.Code)
}

// OnMessage implements server.Handler.
func (d *Daemon) OnMessage(id server.ConnID, m proto.Msg) (*proto.Msg, bool) {
	switch m.T {
	case proto.THello:
		if m.V > proto.Version || m.V == 0 {
			return &proto.Msg{T: proto.TError, Err: proto.ErrUnsupportedVersion,
				Message: fmt.Sprintf("daemon speaks protocol v%d", proto.Version)}, true
		}
		mode := parseMode(m.Mode, d.log)
		d.store.Hello(id, state.HelloInfo{
			Kind: m.Client, App: m.App, PID: m.PID, Mode: mode, Focus: toFocus(m.Focused),
			Nested: m.Nested, TTL: time.Duration(m.TTLMs) * time.Millisecond,
		})
		d.mu.Lock()
		d.clients[id] = true
		d.mu.Unlock()
		d.log.Info("client connected", "conn", id, "kind", m.Client, "app", m.App, "pid", m.PID, "plugin", m.Plugin, "tmux", m.Tmux, "nested", m.Nested)
		dec := d.store.Decision()
		return &proto.Msg{V: proto.Version, T: proto.TWelcome, Daemon: d.o.Version, Code: proto.U8(dec.Code)}, false

	case proto.TMode:
		if !d.isClient(id) {
			return nil, false
		}
		d.store.SetMode(id, parseMode(m.Mode, d.log), time.Duration(m.TTLMs)*time.Millisecond)
		return nil, false

	case proto.TFocus:
		if !d.isClient(id) {
			return nil, false
		}
		var mp *state.Mode
		if m.Mode != "" {
			mm := parseMode(m.Mode, d.log)
			mp = &mm
		}
		d.store.SetFocus(id, toFocus(m.Focused), mp)
		return nil, false

	case proto.TBye:
		d.forget(id)
		return nil, true

	case proto.TSet:
		if m.V > proto.Version || m.V == 0 {
			return &proto.Msg{T: proto.TError, Err: proto.ErrUnsupportedVersion}, true
		}
		return d.handleSet(m), true

	case proto.TStatus:
		return &proto.Msg{T: proto.TStatus, Status: d.status()}, true

	case proto.TDevices:
		return &proto.Msg{T: proto.TDevices, Devices: d.rec.Devices()}, true
	}
	d.log.Debug("ignoring unknown message type", "t", m.T, "conn", id)
	return nil, false
}

// OnClose implements server.Handler.
func (d *Daemon) OnClose(id server.ConnID) { d.forget(id) }

func (d *Daemon) isClient(id server.ConnID) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.clients[id]
}

func (d *Daemon) forget(id server.ConnID) {
	d.mu.Lock()
	was := d.clients[id]
	delete(d.clients, id)
	d.mu.Unlock()
	if was {
		d.log.Info("client disconnected", "conn", id)
		d.store.Gone(id)
	}
}

func (d *Daemon) handleSet(m proto.Msg) *proto.Msg {
	var active bool
	switch mode := m.Mode; mode {
	case "auto", "":
		d.store.SetOverride(nil)
	default:
		mm, ok := state.ParseMode(mode)
		if !ok || mm == state.None {
			return &proto.Msg{T: proto.TError, Err: proto.ErrBadRequest, Message: "unknown mode " + mode}
		}
		o := &state.Override{Mode: mm, Sticky: m.Sticky}
		if m.TTLMs > 0 {
			o.Until = time.Now().Add(time.Duration(m.TTLMs) * time.Millisecond)
		}
		active = d.store.SetOverride(o)
	}
	dec := d.store.Decision()
	reason := dec.Reason
	if !active && m.Mode != "auto" && m.Mode != "" {
		reason = "override toggled off; " + reason
	}
	return &proto.Msg{T: proto.TOK, Code: proto.U8(dec.Code), Mode: dec.Mode.String(), Reason: reason}
}

func (d *Daemon) status() *proto.Status {
	snap := d.store.Snapshot()
	dec := d.store.Decision()
	st := &proto.Status{
		Code: dec.Code, Mode: dec.Mode.String(), Reason: dec.Reason,
		Frontmost: &proto.FrontmostInfo{Known: snap.Frontmost.Known, Class: snap.Frontmost.Class, PID: snap.Frontmost.PID},
		Clients:   []proto.ClientInfo{},
		Devices:   d.rec.Devices(),
		UptimeS:   int64(time.Since(d.started).Seconds()),
	}
	if d.log.Enabled(context.Background(), slog.LevelDebug) {
		st.Frontmost.Title = snap.Frontmost.Title
	}
	for _, c := range snap.Clients {
		ci := proto.ClientInfo{ID: c.ID, Kind: c.Kind, App: c.App, PID: c.PID, Mode: c.Mode.String(),
			Nested: c.Nested, IdleMs: time.Since(c.LastEvent).Milliseconds()}
		switch c.Focus {
		case state.FocusYes:
			ci.Focused = proto.Bool(true)
		case state.FocusNo:
			ci.Focused = proto.Bool(false)
		}
		if !c.Deadline.IsZero() {
			ci.TTLLeftMs = time.Until(c.Deadline).Milliseconds()
		}
		st.Clients = append(st.Clients, ci)
	}
	if o := snap.Override; o != nil && o.Active(snap.Now) {
		oi := &proto.OverrideInfo{Mode: o.Mode.String(), Sticky: o.Sticky}
		if !o.Until.IsZero() {
			oi.LeftMs = time.Until(o.Until).Milliseconds()
		}
		st.Override = oi
	}
	return st
}

// parseMode maps a client's wire mode. Empty is None (no opinion), which is
// what the VSCode companion sends while the text editor has focus.
func parseMode(s string, log *slog.Logger) state.Mode {
	m, ok := state.ParseMode(s)
	if !ok {
		log.Debug("unknown mode from client, treating as normal", "mode", s)
	}
	return m
}

func toFocus(b *bool) state.Focus {
	switch {
	case b == nil:
		return state.FocusUnknown
	case *b:
		return state.FocusYes
	default:
		return state.FocusNo
	}
}
