package atspi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/dbus"
	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
)

const (
	a11yBusName     = "org.a11y.Bus"
	a11yBusPath     = "/org/a11y/bus"
	a11yBusIface    = "org.a11y.Bus"
	statusIface     = "org.a11y.Status"
	registryName    = "org.a11y.atspi.Registry"
	registryPath    = "/org/a11y/atspi/registry"
	registryIface   = "org.a11y.atspi.Registry"
	accessibleIface = "org.a11y.atspi.Accessible"
	eventIface      = "org.a11y.atspi.Event.Object"
	nullPath        = "/org/a11y/atspi/null"
	focusEvent      = "object:state-changed:focused"
	matchRule       = "type='signal',interface='" + eventIface + "',member='StateChanged'"
)

// Watcher implements focus.WidgetWatcher on top of the AT-SPI2 bus.
//
// Applications only emit accessibility events while a listener is registered
// from a live connection, and Chromium only builds its accessibility tree when
// org.a11y.Status.IsEnabled is true at startup -- so Run keeps one connection
// open for its whole life and Enable flips the flag for every toolkit on the
// session, which is what a screen reader would do.
type Watcher struct {
	log   *slog.Logger
	Retry time.Duration
	// Depth bounds how many exposed ancestors are inspected for a widget marker.
	Depth int
	// CallTimeout bounds each query to the focused application.
	CallTimeout time.Duration
}

// New creates a Watcher.
func New(log *slog.Logger) *Watcher {
	if log == nil {
		log = slog.Default()
	}
	return &Watcher{log: log, Retry: 3 * time.Second, Depth: 6, CallTimeout: time.Second}
}

// Enabled reports whether both org.a11y.Status flags -- IsEnabled and
// ScreenReaderEnabled -- are set on the session bus.
func Enabled(ctx context.Context) (bool, error) {
	c, err := dbus.Dial(ctx, dbus.SessionBusAddress())
	if err != nil {
		return false, err
	}
	defer c.Close()
	for _, prop := range []string{"IsEnabled", "ScreenReaderEnabled"} {
		v, err := c.GetProperty(ctx, a11yBusName, a11yBusPath, statusIface, prop)
		if err != nil {
			return false, err
		}
		b, ok := v.Value.(bool)
		if !ok {
			return false, fmt.Errorf("%s is %T", prop, v.Value)
		}
		if !b {
			return false, nil
		}
	}
	return true, nil
}

// Enable sets both org.a11y.Status flags, as a screen reader does. GTK and Qt
// read IsEnabled at startup; Chromium exposes its web content only with
// ScreenReaderEnabled, which it also picks up at runtime.
func Enable(ctx context.Context) error {
	c, err := dbus.Dial(ctx, dbus.SessionBusAddress())
	if err != nil {
		return err
	}
	defer c.Close()
	for _, prop := range []string{"IsEnabled", "ScreenReaderEnabled"} {
		if err := c.SetProperty(ctx, a11yBusName, a11yBusPath, statusIface, prop, dbus.Variant{Sig: "b", Value: true}); err != nil {
			return fmt.Errorf("%s: %w", prop, err)
		}
	}
	return nil
}

// Run streams widget focus until ctx is done, reconnecting when the bus goes away.
func (w *Watcher) Run(ctx context.Context, emit func(focus.Widget)) error {
	return w.run(ctx, func(pid int, _ Focused, v Verdict) {
		if v.Ignore {
			return
		}
		emit(focus.Widget{PID: pid, Editor: v.Editor, Detail: v.Detail})
	})
}

// Event is one line of `zmk-vim-mode atspi-watch`.
type Event struct {
	PID       int      `json:"pid"`
	Role      string   `json:"role"`
	Name      string   `json:"name,omitempty"`
	Tag       string   `json:"tag,omitempty"`
	Class     string   `json:"class,omitempty"`
	RoleDesc  string   `json:"roledescription,omitempty"`
	Ancestors []string `json:"ancestors,omitempty"`
	// Attrs is everything the application exposed, whatever the keys: the
	// classifier's assumptions about them are checked against this.
	Attrs   map[string]string `json:"attrs,omitempty"`
	Editor  bool              `json:"editor"`
	Ignored bool              `json:"ignored,omitempty"`
	Detail  string            `json:"detail"`
}

// Watch prints every focus event as JSON, classification included -- the tool
// for tuning the rules on a real desktop.
func (w *Watcher) Watch(ctx context.Context, out io.Writer) error {
	enc := json.NewEncoder(out)
	return w.run(ctx, func(pid int, f Focused, v Verdict) {
		_ = enc.Encode(Event{
			PID: pid, Role: f.Role, Name: f.Name, Tag: f.Attrs["tag"], Class: f.Attrs["class"],
			RoleDesc: f.Attrs["roledescription"], Ancestors: f.AncestorClasses, Attrs: f.Attrs,
			Editor: v.Editor, Ignored: v.Ignore, Detail: v.Detail,
		})
	})
}

func (w *Watcher) run(ctx context.Context, on func(pid int, f Focused, v Verdict)) error {
	for ctx.Err() == nil {
		conn, err := w.connect(ctx)
		if err != nil {
			w.log.Warn("accessibility bus unavailable; retrying", "err", err, "in", w.Retry)
			if !sleep(ctx, w.Retry) {
				break
			}
			continue
		}
		w.log.Info("listening on the accessibility bus", "name", conn.Name)
		err = w.stream(ctx, conn, on)
		conn.Close()
		if ctx.Err() != nil {
			break
		}
		w.log.Info("accessibility bus connection lost; reconnecting", "err", err)
		sleep(ctx, w.Retry)
	}
	return nil
}

// connect finds the accessibility bus through the session bus, makes sure the
// toolkits will talk, and registers as a focus listener.
func (w *Watcher) connect(ctx context.Context) (*dbus.Conn, error) {
	dctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	session, err := dbus.Dial(dctx, dbus.SessionBusAddress())
	if err != nil {
		return nil, fmt.Errorf("session bus: %w", err)
	}
	defer session.Close()
	reply, err := session.Call(dctx, a11yBusName, a11yBusPath, a11yBusIface, "GetAddress", "")
	if err != nil {
		return nil, fmt.Errorf("org.a11y.Bus.GetAddress: %w", err)
	}
	addr := ""
	if len(reply) == 1 {
		addr, _ = dbus.String(reply[0])
	}
	if addr == "" {
		return nil, errors.New("org.a11y.Bus.GetAddress returned no address")
	}
	// Both flags, as a screen reader sets them: GTK/Qt look at IsEnabled, but
	// Chromium exposes its *web content* (VSCode's whole UI) only when
	// ScreenReaderEnabled is on, and it watches that one at runtime.
	for _, prop := range []string{"IsEnabled", "ScreenReaderEnabled"} {
		v, err := session.GetProperty(dctx, a11yBusName, a11yBusPath, statusIface, prop)
		if err != nil {
			w.log.Warn("cannot read org.a11y.Status", "property", prop, "err", err)
			continue
		}
		if on, _ := v.Value.(bool); on {
			w.log.Info("accessibility flag already set", "property", prop)
			continue
		}
		if err := session.SetProperty(dctx, a11yBusName, a11yBusPath, statusIface, prop, dbus.Variant{Sig: "b", Value: true}); err != nil {
			w.log.Warn("cannot set org.a11y.Status flag; applications may stay silent", "property", prop, "err", err)
		} else {
			w.log.Info("set accessibility flag on the session", "property", prop)
		}
	}

	a11y, err := dbus.Dial(dctx, addr)
	if err != nil {
		return nil, fmt.Errorf("accessibility bus %s: %w", addr, err)
	}
	if err := a11y.AddMatch(dctx, matchRule); err != nil {
		a11y.Close()
		return nil, fmt.Errorf("AddMatch: %w", err)
	}
	if _, err := a11y.Call(dctx, registryName, registryPath, registryIface, "RegisterEvent", "s", focusEvent); err != nil {
		a11y.Close()
		return nil, fmt.Errorf("RegisterEvent: %w", err)
	}
	return a11y, nil
}

func (w *Watcher) stream(ctx context.Context, conn *dbus.Conn, on func(int, Focused, Verdict)) error {
	pids := map[string]int{} // sender → pid
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sig, ok := <-conn.Signals():
			if !ok {
				return conn.Err()
			}
			if sig.Interface != eventIface || sig.Member != "StateChanged" || len(sig.Body) < 2 {
				continue
			}
			detail, _ := dbus.String(sig.Body[0])
			set, _ := sig.Body[1].(int32)
			if detail != "focused" || set != 1 {
				continue
			}
			pid, ok := pids[sig.Sender]
			if !ok {
				cctx, cancel := context.WithTimeout(ctx, w.CallTimeout)
				pid, _ = conn.ConnectionPID(cctx, sig.Sender)
				cancel()
				pids[sig.Sender] = pid
			}
			f := w.inspect(ctx, conn, sig.Sender, sig.Path)
			v := ClassifyVSCode(f)
			w.log.Debug("a11y focus", "pid", pid, "role", f.Role, "tag", f.Attrs["tag"], "class", f.Attrs["class"],
				"editor", v.Editor, "ignored", v.Ignore, "detail", v.Detail)
			on(pid, f, v)
		}
	}
}

// inspect gathers what the classifier needs about one accessible object. Every
// query is best effort: a vanished object just yields fewer facts, and the
// classifier's default for "unknown" is raw.
func (w *Watcher) inspect(ctx context.Context, conn *dbus.Conn, sender, path string) Focused {
	f := Focused{Attrs: map[string]string{}}
	call := func(method string) ([]any, error) {
		cctx, cancel := context.WithTimeout(ctx, w.CallTimeout)
		defer cancel()
		return conn.Call(cctx, sender, path, accessibleIface, method, "")
	}
	if r, err := call("GetRoleName"); err == nil && len(r) == 1 {
		f.Role, _ = dbus.String(r[0])
	}
	if r, err := call("GetAttributes"); err == nil && len(r) == 1 {
		f.Attrs = dbus.StringMap(r[0])
	}
	cctx, cancel := context.WithTimeout(ctx, w.CallTimeout)
	if v, err := conn.GetProperty(cctx, sender, path, accessibleIface, "Name"); err == nil {
		f.Name, _ = dbus.String(v)
	}
	cancel()
	// Ancestors: only worth walking for Monaco text areas, where a widget
	// marker changes the verdict.
	if strings.Contains(f.Attrs["class"], "inputarea") || strings.EqualFold(f.Attrs["roledescription"], "editor") {
		cur := path
		for i := 0; i < w.Depth; i++ {
			cctx, cancel := context.WithTimeout(ctx, w.CallTimeout)
			v, err := conn.GetProperty(cctx, sender, cur, accessibleIface, "Parent")
			cancel()
			if err != nil {
				break
			}
			st, ok := dbus.Struct(v)
			if !ok || len(st) != 2 {
				break
			}
			ppath, _ := dbus.String(st[1])
			if ppath == "" || ppath == nullPath {
				break
			}
			cctx, cancel = context.WithTimeout(ctx, w.CallTimeout)
			r, err := conn.Call(cctx, sender, ppath, accessibleIface, "GetAttributes", "")
			cancel()
			if err == nil && len(r) == 1 {
				f.AncestorClasses = append(f.AncestorClasses, dbus.StringMap(r[0])["class"])
			}
			cur = ppath
		}
	}
	return f
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
