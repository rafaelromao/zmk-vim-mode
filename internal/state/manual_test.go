package state

import "testing"

func TestNewNvimSessionClearsOffOverride(t *testing.T) {
	s := NewStore(DefaultRules(), nil)
	s.SetFrontmost(app("ghostty"))
	existing := HelloInfo{Kind: "nvim", PID: 101, Session: "first-process", Mode: Normal, Focus: FocusYes}
	s.Hello(1, existing)
	s.SetOverride(&Override{Mode: Off, Sticky: true}, false)
	assertOff := func(step string) {
		t.Helper()
		if d := s.Decision(); d.Mode != Off || s.Snapshot().Override == nil {
			t.Fatalf("%s must preserve manual disable: %+v", step, d)
		}
	}
	s.SetMode(1, Insert, 0)
	s.SetFocus(1, FocusYes, nil)
	assertOff("existing client update")
	s.Hello(1, existing)
	assertOff("repeated hello")
	s.Gone(1)
	s.Hello(2, existing)
	assertOff("existing process reconnect")
	s.Hello(3, HelloInfo{Kind: "vscode", PID: 102, Mode: Normal})
	assertOff("non-Neovim client")
	// The OS may reuse the old PID; a new token still means a new session.
	s.Hello(4, HelloInfo{Kind: "nvim", PID: 101, Session: "replacement-process", Mode: Normal, Focus: FocusYes})
	if d := s.Decision(); d.Mode != Normal || d.ClientID != 4 || s.Snapshot().Override != nil {
		t.Fatalf("new Neovim session must restore automatic tracking: %+v", d)
	}
}

func TestNewNvimSessionPreservesOtherOverrides(t *testing.T) {
	for _, mode := range []Mode{Legacy, Raw, Insert} {
		s := NewStore(DefaultRules(), nil)
		s.SetFrontmost(app("ghostty"))
		s.SetOverride(&Override{Mode: mode, Sticky: true}, false)
		s.Hello(1, HelloInfo{Kind: "nvim", PID: 101, Mode: Normal, Focus: FocusYes})
		if d := s.Decision(); d.Mode != mode || s.Snapshot().Override == nil {
			t.Fatalf("new session must preserve %s override: %+v", mode, d)
		}
	}
}

func TestHostManualLifecycleParity(t *testing.T) {
	for _, class := range []string{"ghostty", "com.mitchellh.ghostty", "com.apple.terminal", "com.googlecode.iterm2", "com.neovide.neovide"} {
		t.Run(class, func(t *testing.T) {
			s := NewStore(DefaultRules(), nil)
			s.SetFrontmost(app(class))
			check := func(step string, want Mode) {
				t.Helper()
				if d := s.Decision(); d.Mode != want {
					t.Fatalf("%s: got %+v, want %s", step, d, want)
				}
			}
			check("automatic without client", Off)
			s.SetOverride(&Override{Mode: Legacy, FallbackLegacy: true}, false)
			check("manual fallback", Legacy)
			h := HelloInfo{Kind: "nvim", PID: 101, Session: "first", Mode: Normal, Focus: FocusYes}
			s.Hello(1, h)
			for _, mode := range []Mode{Normal, Insert, Visual, Cmdline, Raw, Off} {
				s.SetMode(1, mode, 0)
				check("manual follows explicit "+mode.String(), mode)
			}
			s.SetOverride(&Override{Mode: Off, Sticky: true}, false)
			s.SetMode(1, Normal, 0)
			check("disabled blocks editor update", Off)
			s.Gone(1)
			s.Hello(2, h)
			check("disabled blocks same-session reconnect", Off)
			s.Gone(2)
			h.Session = "second"
			s.Hello(3, h)
			check("new session restores automatic", Normal)
			if s.Snapshot().Override != nil {
				t.Fatal("new session must clear off override")
			}
			s.SetMode(3, Insert, 0)
			check("automatic follows insert", Insert)
			s.SetOverride(&Override{Mode: Off, Sticky: true}, false)
			s.SetFrontmost(app("browser"))
			check("disabled survives application switch", Off)
			s.SetOverride(&Override{Mode: Legacy, FallbackLegacy: true}, false)
			check("enable replaces disable", Legacy)
			s.SetFrontmost(app(class))
			check("manual clears on application switch", Insert)
		})
	}
}

func TestManualAndLegacyAreDistinctOverrides(t *testing.T) {
	for _, manualFirst := range []bool{false, true} {
		s := NewStore(DefaultRules(), nil)
		s.SetFrontmost(app("ghostty"))
		s.Hello(1, HelloInfo{Kind: "nvim", Mode: Insert, Focus: FocusYes})
		first := &Override{Mode: Legacy, FallbackLegacy: manualFirst}
		second := &Override{Mode: Legacy, FallbackLegacy: !manualFirst}
		s.SetOverride(first, true)
		if !s.SetOverride(second, true) {
			t.Fatalf("switching from manual=%v must replace, not toggle off", manualFirst)
		}
		want := Legacy
		if second.FallbackLegacy {
			want = Insert
		}
		if got := s.Decision(); got.Mode != want {
			t.Fatalf("switching from manual=%v: got %+v, want %s", manualFirst, got, want)
		}
		if s.SetOverride(second, true) {
			t.Fatal("repeating the same override should still toggle off")
		}
	}
}
