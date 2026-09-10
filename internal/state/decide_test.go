package state

import (
	"testing"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
)

var t0 = time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)

func app(class string) focus.App { return focus.App{Known: true, Class: class, PID: 100} }

func nvim(id uint64, m Mode, f Focus, focusSeq, eventSeq uint64) Client {
	return Client{ID: id, Kind: "nvim", Mode: m, Focus: f, FocusSeq: focusSeq, EventSeq: eventSeq, LastEvent: t0}
}

// companion is the VSCode extension: the app's own client, never focused/unfocused.
func companion(id uint64, m Mode, eventSeq uint64, deadline time.Time) Client {
	return Client{ID: id, Kind: "vscode", App: "vscode", Mode: m, EventSeq: eventSeq, Deadline: deadline, LastEvent: t0}
}

// embedded is the Neovim that vscode-neovim runs inside VSCode.
var embedded = Client{ID: 9, Kind: "nvim", App: "vscode", Mode: Insert, EventSeq: 3, LastEvent: t0}

func vscodeTitled(title string) focus.App {
	return focus.App{Known: true, Class: "Code", Title: title, PID: 100}
}

func TestDecide(t *testing.T) {
	r := DefaultRules()
	ghostty := app("com.mitchellh.ghostty")
	browser := app("firefox")
	vscode := app("Code")
	neovide := app("neovide")

	cases := []struct {
		name     string
		snap     Snapshot
		wantMode Mode
		wantCode uint8
		wantCli  uint64
	}{
		{"frontmost unknown + focused client → client mode (fail open)",
			Snapshot{Frontmost: focus.App{}, Clients: []Client{nvim(1, Insert, FocusYes, 1, 1)}}, Insert, CodeInsert, 1},
		{"frontmost unknown, no clients → off",
			Snapshot{Frontmost: focus.App{}}, Off, CodeOff, 0},
		{"browser + focused client → off (veto)",
			Snapshot{Frontmost: browser, Clients: []Client{nvim(1, Insert, FocusYes, 1, 1)}}, Off, CodeOff, 0},
		{"vscode, no vscode client → legacy",
			Snapshot{Frontmost: vscode, Clients: []Client{nvim(1, Insert, FocusYes, 1, 1)}}, Legacy, CodeLegacy, 0},
		{"vscode with vscode client → client mode",
			Snapshot{Frontmost: vscode, Clients: []Client{{ID: 9, Kind: "nvim", App: "vscode", Mode: Visual, Focus: FocusUnknown, EventSeq: 3}}}, Visual, CodeVisual, 9},
		{"vscode client with lapsed ttl → no opinion → legacy",
			Snapshot{Frontmost: vscode, Now: t0, Clients: []Client{{ID: 9, Kind: "nvim", App: "vscode", Mode: Normal, Deadline: t0.Add(-time.Millisecond), EventSeq: 3}}}, Legacy, CodeLegacy, 0},
		{"vscode client with live ttl → mode",
			Snapshot{Frontmost: vscode, Now: t0, Clients: []Client{{ID: 9, Kind: "nvim", App: "vscode", Mode: Normal, Deadline: t0.Add(time.Second), EventSeq: 3}}}, Normal, CodeNormal, 9},
		{"vscode companion alone, no opinion → legacy",
			Snapshot{Frontmost: vscode, Clients: []Client{companion(5, None, 4, time.Time{})}}, Legacy, CodeLegacy, 0},
		{"vscode companion raw beats embedded nvim",
			Snapshot{Frontmost: vscode, Clients: []Client{embedded, companion(5, Raw, 4, time.Time{})}}, Raw, CodeRaw, 5},
		{"vscode companion raw with live ttl beats embedded nvim",
			Snapshot{Frontmost: vscode, Now: t0, Clients: []Client{embedded, companion(5, Raw, 4, t0.Add(time.Second))}}, Raw, CodeRaw, 5},
		{"vscode companion raw expired → embedded nvim decides",
			Snapshot{Frontmost: vscode, Now: t0, Clients: []Client{embedded, companion(5, Raw, 4, t0.Add(-time.Second))}}, Insert, CodeInsert, 9},
		{"vscode companion none → embedded nvim decides",
			Snapshot{Frontmost: vscode, Clients: []Client{embedded, companion(5, None, 4, time.Time{})}}, Insert, CodeInsert, 9},
		{"vscode title marks a focused view → raw, whatever the clients say",
			Snapshot{Frontmost: vscodeTitled("main.go — zmk [Terminal]"), Clients: []Client{embedded}}, Raw, CodeRaw, 0},
		{"vscode title with empty marker → a widget outside any view (Extensions search) → raw",
			Snapshot{Frontmost: vscodeTitled("main.go — zmk []"), Clients: []Client{embedded}}, Raw, CodeRaw, 0},
		{"vscode title [Text Editor] → the editor has focus → clients decide",
			Snapshot{Frontmost: vscodeTitled("main.go — zmk [Text Editor]"), Clients: []Client{embedded}}, Insert, CodeInsert, 9},
		{"vscode title [text editor] (any case) → clients decide",
			Snapshot{Frontmost: vscodeTitled("main.go — zmk [text editor]"), Clients: []Client{embedded}}, Insert, CodeInsert, 9},
		{"vscode title [terminal] (view id, lower case) → raw",
			Snapshot{Frontmost: vscodeTitled("mouse.dtsi - keyboards [terminal]"), Clients: []Client{embedded}}, Raw, CodeRaw, 0},
		{"vscode title without marker (setting not applied) → clients decide",
			Snapshot{Frontmost: vscodeTitled("main.go — zmk — Visual Studio Code"), Clients: []Client{embedded}}, Insert, CodeInsert, 9},
		{"vscode title marker not at the end is ignored",
			Snapshot{Frontmost: vscodeTitled("[Terminal] main.go — zmk"), Clients: []Client{embedded}}, Insert, CodeInsert, 9},
		{"a11y: focus in a quick input → raw, whatever the clients say",
			Snapshot{Frontmost: vscode, Widget: &focus.Widget{PID: 100, Detail: "input.input"}, Clients: []Client{embedded}}, Raw, CodeRaw, 0},
		{"a11y: focus in the editor overrides the companion's stale raw hint",
			Snapshot{Frontmost: vscode, Widget: &focus.Widget{PID: 100, Editor: true, Detail: "monaco editor"}, Clients: []Client{embedded, companion(5, Raw, 4, time.Time{})}}, Insert, CodeInsert, 9},
		{"a11y: widget of another pid is not this window",
			Snapshot{Frontmost: vscode, Widget: &focus.Widget{PID: 999, Editor: true}, Clients: []Client{embedded, companion(5, Raw, 4, time.Time{})}}, Raw, CodeRaw, 5},
		{"a11y: title tool window still comes first",
			Snapshot{Frontmost: vscodeTitled("x [terminal]"), Widget: &focus.Widget{PID: 100, Editor: true}, Clients: []Client{embedded}}, Raw, CodeRaw, 0},
		{"a11y: apps without a classifier ignore widget focus",
			Snapshot{Frontmost: app("obsidian"), Widget: &focus.Widget{PID: 100, Detail: "entry"}, Clients: []Client{{ID: 3, Kind: "obsidian", App: "obsidian", Mode: Insert, EventSeq: 1}}}, Insert, CodeInsert, 3},
		{"obsidian own client with a mode → that mode",
			Snapshot{Frontmost: app("obsidian"), Clients: []Client{{ID: 3, Kind: "obsidian", App: "obsidian", Mode: Insert, EventSeq: 1}}}, Insert, CodeInsert, 3},
		{"obsidian own client with no opinion → legacy",
			Snapshot{Frontmost: app("obsidian"), Clients: []Client{{ID: 3, Kind: "obsidian", App: "obsidian", Mode: None, EventSeq: 1}}}, Legacy, CodeLegacy, 0},
		{"ghostty, no clients → off",
			Snapshot{Frontmost: ghostty}, Off, CodeOff, 0},
		{"ghostty, one focused insert client → insert",
			Snapshot{Frontmost: ghostty, Clients: []Client{nvim(1, Insert, FocusYes, 1, 1)}}, Insert, CodeInsert, 1},
		{"two focused clients → later focus seq wins",
			Snapshot{Frontmost: ghostty, Clients: []Client{nvim(1, Insert, FocusYes, 5, 9), nvim(2, Normal, FocusYes, 7, 8)}}, Normal, CodeNormal, 2},
		{"A focused=false vs B unknown → B",
			Snapshot{Frontmost: ghostty, Clients: []Client{nvim(1, Insert, FocusNo, 9, 9), nvim(2, Cmdline, FocusUnknown, 0, 1)}}, Cmdline, CodeCmdline, 2},
		{"focused=yes beats unknown even with older seq",
			Snapshot{Frontmost: ghostty, Clients: []Client{nvim(1, Insert, FocusYes, 1, 1), nvim(2, Normal, FocusUnknown, 0, 9)}}, Insert, CodeInsert, 1},
		{"nested tie-break: inner nvim wins on equal focus seq",
			Snapshot{Frontmost: ghostty, Clients: []Client{
				{ID: 1, Kind: "nvim", Mode: Insert, Focus: FocusYes, FocusSeq: 4, EventSeq: 4},
				{ID: 2, Kind: "nvim", Mode: Normal, Focus: FocusYes, FocusSeq: 4, EventSeq: 3, Nested: true}}}, Normal, CodeNormal, 2},
		{"only client sent focus false (VimSuspend) → off",
			Snapshot{Frontmost: ghostty, Clients: []Client{nvim(1, Insert, FocusNo, 1, 1)}}, Off, CodeOff, 0},
		{"override wins over browser",
			Snapshot{Frontmost: browser, Now: t0, Override: &Override{Mode: Legacy}}, Legacy, CodeLegacy, 0},
		{"override with ttl expired → ignored",
			Snapshot{Frontmost: browser, Now: t0, Override: &Override{Mode: Legacy, Until: t0.Add(-time.Second)}}, Off, CodeOff, 0},
		{"override raw over ghostty client",
			Snapshot{Frontmost: ghostty, Now: t0, Override: &Override{Mode: Raw, Until: t0.Add(time.Second)}, Clients: []Client{nvim(1, Insert, FocusYes, 1, 1)}}, Raw, CodeRaw, 0},
		{"title heuristic: ghostty title contains nvim, no client → legacy",
			Snapshot{Frontmost: focus.App{Known: true, Class: "ghostty", Title: "nvim ~/x.go"}}, Legacy, CodeLegacy, 0},
		{"title heuristic off when a client exists",
			Snapshot{Frontmost: focus.App{Known: true, Class: "ghostty", Title: "nvim"}, Clients: []Client{nvim(1, Insert, FocusYes, 1, 1)}}, Insert, CodeInsert, 1},
		{"neovide (gui nvim app) + client → client mode",
			Snapshot{Frontmost: neovide, Clients: []Client{nvim(1, Visual, FocusUnknown, 0, 1)}}, Visual, CodeVisual, 1},
		{"jetbrains glob → legacy",
			Snapshot{Frontmost: app("jetbrains-idea")}, Legacy, CodeLegacy, 0},
		{"obsidian with obsidian client raw → raw",
			Snapshot{Frontmost: app("obsidian"), Clients: []Client{{ID: 3, Kind: "obsidian", App: "obsidian", Mode: Raw, EventSeq: 1}}}, Raw, CodeRaw, 3},
		{"terminal-hosted client does not serve a vscode window",
			Snapshot{Frontmost: vscode, Clients: []Client{nvim(1, Insert, FocusYes, 1, 1)}}, Legacy, CodeLegacy, 0},
	}
	for _, tc := range cases {
		snap := tc.snap
		if snap.Now.IsZero() {
			snap.Now = t0
		}
		d := Decide(r, snap)
		if d.Mode != tc.wantMode || d.Code != tc.wantCode || d.ClientID != tc.wantCli {
			t.Errorf("%s: got mode=%s code=%d client=%d (%s); want mode=%s code=%d client=%d",
				tc.name, d.Mode, d.Code, d.ClientID, d.Reason, tc.wantMode, tc.wantCode, tc.wantCli)
		}
	}
}

func TestTitleHeuristicDisabled(t *testing.T) {
	r := DefaultRules()
	r.TitleLegacy = nil
	d := Decide(r, Snapshot{Frontmost: focus.App{Known: true, Class: "ghostty", Title: "nvim"}, Now: t0})
	if d.Mode != Off {
		t.Fatalf("expected off, got %s", d.Mode)
	}
}

func TestCodesAndAliases(t *testing.T) {
	if Legacy.Code() != 4 || Raw.Code() != 6 || Cmdline.Code() != 5 || Off.Code() != 0 {
		t.Fatal("code table changed")
	}
	if SilentAlias(CodeLegacy) != CodeLegacySilent || SilentAlias(CodeInsert) != CodeInsert {
		t.Fatal("alias")
	}
	for _, s := range []string{"off", "normal", "insert", "visual", "cmdline", "raw", "legacy", "none"} {
		m, ok := ParseMode(s)
		if !ok || m.String() != s {
			t.Fatalf("parse %s → %s %v", s, m, ok)
		}
	}
	if m, ok := ParseMode(""); !ok || m != None || m.Code() != CodeOff || m.Opinion() {
		t.Fatalf("empty mode must be None with no opinion: %s %v", m, ok)
	}
	if m, ok := ParseMode("OP_PENDING"); ok || m != Normal {
		t.Fatalf("unknown modes must map to normal,false: %s %v", m, ok)
	}
	if m, ok := ParseMode("replace"); !ok || m != Insert {
		t.Fatalf("replace must map to insert: %s %v", m, ok)
	}
}

func TestStore(t *testing.T) {
	var got []Decision
	s := NewStore(DefaultRules(), func(d Decision) { got = append(got, d) })
	now := t0
	s.SetClock(func() time.Time { return now })

	s.SetFrontmost(app("ghostty"))
	if len(got) != 0 { // initial decision is Off; no change from implicit initial
		// first computation counts as a change only if code differs from nothing: we expect one emit
	}
	s.Hello(1, HelloInfo{Kind: "nvim", Mode: Normal, Focus: FocusYes})
	if d := s.Decision(); d.Mode != Normal || d.ClientID != 1 {
		t.Fatalf("after hello: %+v", d)
	}
	s.SetMode(1, Insert, 0)
	if d := s.Decision(); d.Mode != Insert {
		t.Fatalf("after mode: %+v", d)
	}
	// second client in another pane takes focus
	s.Hello(2, HelloInfo{Kind: "nvim", Mode: Visual, Focus: FocusYes})
	if d := s.Decision(); d.ClientID != 2 || d.Mode != Visual {
		t.Fatalf("second focused client should win: %+v", d)
	}
	f := FocusNo
	_ = f
	s.SetFocus(2, FocusNo, nil)
	if d := s.Decision(); d.ClientID != 1 || d.Mode != Insert {
		t.Fatalf("back to first client: %+v", d)
	}
	// override, then frontmost change clears it
	s.SetOverride(&Override{Mode: Raw})
	if d := s.Decision(); d.Mode != Raw {
		t.Fatalf("override: %+v", d)
	}
	s.SetFrontmost(app("firefox"))
	if d := s.Decision(); d.Mode != Off {
		t.Fatalf("override should clear on app change: %+v", d)
	}
	// sticky override survives, same-mode re-issue toggles off
	s.SetOverride(&Override{Mode: Legacy, Sticky: true})
	s.SetFrontmost(app("ghostty"))
	if d := s.Decision(); d.Mode != Legacy {
		t.Fatalf("sticky override should survive: %+v", d)
	}
	if active := s.SetOverride(&Override{Mode: Legacy, Sticky: true}); active {
		t.Fatal("re-issuing same override should toggle back to auto")
	}
	if d := s.Decision(); d.Mode != Insert {
		t.Fatalf("auto again: %+v", d)
	}
	// TTL lapse via clock + Recompute
	s.SetMode(1, Normal, 300*time.Millisecond)
	if d := s.Decision(); d.Mode != Normal {
		t.Fatalf("ttl live: %+v", d)
	}
	dl, ok := s.NextDeadline()
	if !ok || !dl.Equal(now.Add(300*time.Millisecond)) {
		t.Fatalf("deadline: %v %v", dl, ok)
	}
	// Lapsed: client 1 has no opinion and client 2 said "not focused" → nobody
	// speaks for this terminal.
	now = now.Add(time.Second)
	s.Recompute()
	if d := s.Decision(); d.Mode != Off {
		t.Fatalf("ttl lapsed → no opinion → off: %+v", d)
	}
	// A "none" report retracts an opinion explicitly.
	s.SetMode(1, Insert, 0)
	if d := s.Decision(); d.Mode != Insert {
		t.Fatalf("fresh report: %+v", d)
	}
	s.SetMode(1, None, 0)
	if d := s.Decision(); d.Mode != Off {
		t.Fatalf("none → off: %+v", d)
	}
	s.Gone(1)
	s.Gone(2)
	if d := s.Decision(); d.Mode != Off {
		t.Fatalf("no clients → off: %+v", d)
	}
	// Accessibility-bus widget focus, joined on the frontmost pid.
	s.SetFrontmost(app("Code"))
	s.Hello(3, HelloInfo{Kind: "nvim", App: "vscode", Mode: Normal})
	if d := s.Decision(); d.Mode != Normal {
		t.Fatalf("embedded nvim: %+v", d)
	}
	if !s.SetWidget(focus.Widget{PID: 100, Detail: "input.input"}) {
		t.Fatal("first widget report is a change")
	}
	if d := s.Decision(); d.Mode != Raw {
		t.Fatalf("quick input focused → raw: %+v", d)
	}
	if s.SetWidget(focus.Widget{PID: 100, Detail: "tree item"}) {
		t.Fatal("other → other is not an editor-focus change")
	}
	if !s.SetWidget(focus.Widget{PID: 100, Editor: true, Detail: "monaco editor"}) {
		t.Fatal("back to the editor is a change")
	}
	if d := s.Decision(); d.Mode != Normal {
		t.Fatalf("editor focused → nvim mode again: %+v", d)
	}
	s.SetWidget(focus.Widget{PID: 4242, Detail: "somewhere else"}) // another app: no effect here
	if d := s.Decision(); d.Mode != Normal {
		t.Fatalf("other pid must not matter: %+v", d)
	}
	s.Gone(3)
	if len(got) == 0 {
		t.Fatal("onChange never called")
	}
	for i := 1; i < len(got); i++ {
		if got[i].Code == got[i-1].Code && got[i].Mode == got[i-1].Mode {
			t.Fatalf("onChange emitted without change at %d: %+v", i, got[i])
		}
	}
}
