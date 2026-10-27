package state

import "testing"

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
