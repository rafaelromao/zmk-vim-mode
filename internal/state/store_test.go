package state

import "testing"

// VSCode's "Save changes?" prompt is a native window of the same process
// whose title has no view marker; once the marker has been seen for that
// process, the prompt is raw even without the accessibility bus.
func TestUnmarkedWindowOfMarkedProcessIsRaw(t *testing.T) {
	s := NewStore(DefaultRules(), nil)
	s.Hello(1, HelloInfo{Kind: "nvim", App: "vscode", PID: 7, Mode: Normal})
	s.SetFrontmost(vscodeTitled("main.go - zmk [Text Editor]"))
	if d := s.Decision(); d.Mode != Normal {
		t.Fatalf("editor window: got %s (%s), want normal", d.Mode, d.Reason)
	}
	s.SetFrontmost(vscodeTitled("Visual Studio Code"))
	if d := s.Decision(); d.Mode != Raw {
		t.Fatalf("dialog window: got %s (%s), want raw", d.Mode, d.Reason)
	}
}
