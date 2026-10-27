package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
	"github.com/rafaelromao/zmk-vim-mode/internal/focus/noop"
	"github.com/rafaelromao/zmk-vim-mode/internal/leds"
	"github.com/rafaelromao/zmk-vim-mode/internal/proto"
	"github.com/rafaelromao/zmk-vim-mode/internal/state"
)

// nullBackend has no devices, unlike leds/null, so the reconciler's coalesced
// writes never run (or log) behind the test's back.
type nullBackend struct{}

func (nullBackend) Start(context.Context, chan<- leds.Event) error { return nil }
func (nullBackend) Devices() []leds.Device                         { return nil }
func (nullBackend) Write(leds.DeviceID, uint8) error               { return nil }

// lastSet returns the attributes of the most recent "set" record in buf.
func lastSet(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var rec map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var r map[string]any
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		if r["msg"] == "set" {
			rec = r
		}
	}
	if rec == nil {
		t.Fatalf("no set record in:\n%s", buf)
	}
	return rec
}

func TestHandleSetLogsTheRequest(t *testing.T) {
	var buf bytes.Buffer
	d, err := New(Options{
		Rules: state.DefaultRules(), Focus: noop.Watcher{}, Backend: nullBackend{},
		Log: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})),
	})
	if err != nil {
		t.Fatal(err)
	}
	const title = "zsh — ~/secret-project"
	d.store.SetFrontmost(focus.App{Known: true, Class: "com.mitchellh.ghostty", Title: title, PID: 100})

	steps := []struct {
		name string
		msg  proto.Msg
		want map[string]any
	}{
		{"sticky legacy, no toggle",
			proto.Msg{V: 1, T: proto.TSet, Mode: "legacy", Sticky: true, NoToggle: true},
			map[string]any{"requested": "legacy", "sticky": true, "no_toggle": true, "ttl_ms": 0.0,
				"override": true, "frontmost": "com.mitchellh.ghostty", "mode": "legacy", "reason": "override"}},
		{"same mode again toggles it off",
			proto.Msg{V: 1, T: proto.TSet, Mode: "legacy", TTLMs: 30000},
			map[string]any{"requested": "legacy", "sticky": false, "no_toggle": false, "ttl_ms": 30000.0,
				"override": false, "mode": "off"}},
		{"manual fallback in a terminal",
			proto.Msg{V: 1, T: proto.TSet, Mode: "manual", NoToggle: true},
			map[string]any{"requested": "manual", "sticky": false, "no_toggle": true, "ttl_ms": 0.0,
				"override": true, "frontmost": "com.mitchellh.ghostty", "mode": "legacy", "reason": "manual; no editor mode"}},
		{"auto",
			proto.Msg{V: 1, T: proto.TSet, Mode: "auto"},
			map[string]any{"requested": "auto", "override": false, "mode": "off"}},
	}
	for _, s := range steps {
		buf.Reset()
		if r := d.handleSet(s.msg); r.T != proto.TOK {
			t.Fatalf("%s: reply %+v", s.name, r)
		}
		rec := lastSet(t, &buf)
		if rec["level"] != "INFO" {
			t.Errorf("%s: level = %v, want INFO", s.name, rec["level"])
		}
		for k, v := range s.want {
			if rec[k] != v {
				t.Errorf("%s: %s = %v, want %v", s.name, k, rec[k], v)
			}
		}
		if strings.Contains(buf.String(), title) {
			t.Errorf("%s: window title logged at info level:\n%s", s.name, buf.String())
		}
	}
}

func TestManualEnableLifecycle(t *testing.T) {
	d, err := New(Options{Rules: state.DefaultRules(), Focus: noop.Watcher{}, Backend: nullBackend{}})
	if err != nil {
		t.Fatal(err)
	}
	d.store.SetFrontmost(focus.App{Known: true, Class: "com.mitchellh.ghostty", PID: 100})
	set := func(mode string) {
		t.Helper()
		r := d.handleSet(proto.Msg{Mode: mode, NoToggle: true, Sticky: mode == "off"})
		if r.T != proto.TOK {
			t.Fatalf("set %s: %+v", mode, r)
		}
	}
	set("off")
	set("manual")
	if st := d.status(); st.Mode != "legacy" || st.Override == nil || !st.Override.Manual {
		t.Fatalf("manual enable in terminal: %+v", st)
	}
	d.store.Hello(1, state.HelloInfo{Kind: "nvim", Mode: state.Insert, Focus: state.FocusYes})
	if st := d.status(); st.Mode != "insert" || st.Override == nil || !st.Override.Manual {
		t.Fatalf("manual follows editor: %+v", st)
	}
	d.store.SetMode(1, state.Off, 0)
	if st := d.status(); st.Mode != "off" {
		t.Fatalf("explicit editor off must be respected: %+v", st)
	}
	set("off")
	d.store.SetMode(1, state.Normal, 0)
	if st := d.status(); st.Mode != "off" {
		t.Fatalf("disable stays off despite editor updates: %+v", st)
	}
	set("manual")
	if st := d.status(); st.Mode != "normal" {
		t.Fatalf("re-enable follows editor again: %+v", st)
	}
}

func TestNewNvimSessionRestoresTrackingAfterDisable(t *testing.T) {
	d, err := New(Options{Rules: state.DefaultRules(), Focus: noop.Watcher{}, Backend: nullBackend{}})
	if err != nil {
		t.Fatal(err)
	}
	d.store.SetFrontmost(focus.App{Known: true, Class: "com.mitchellh.ghostty", PID: 100})
	d.handleSet(proto.Msg{Mode: "off", Sticky: true, NoToggle: true})
	hello := proto.Msg{V: proto.Version, T: proto.THello, Client: "nvim", PID: 101, Session: "first-process", Mode: "normal", Focused: proto.Bool(true)}
	r, closeConn := d.OnMessage(1, hello)
	if closeConn || r == nil || r.T != proto.TWelcome || d.status().Mode != "normal" || d.status().Override != nil {
		t.Fatalf("new session should restore tracking: reply=%+v status=%+v", r, d.status())
	}
	d.handleSet(proto.Msg{Mode: "off", Sticky: true, NoToggle: true})
	d.OnMessage(1, proto.Msg{T: proto.TMode, Mode: "insert"})
	d.OnClose(1)
	d.OnMessage(2, hello)
	if st := d.status(); st.Mode != "off" || st.Override == nil {
		t.Fatalf("same session update/reconnect should stay disabled: %+v", st)
	}
	d.OnClose(2)
	hello.Session = "replacement-process"
	d.OnMessage(3, hello)
	if st := d.status(); st.Mode != "normal" || st.Override != nil {
		t.Fatalf("replacement Neovim should restore tracking: %+v", st)
	}
}
