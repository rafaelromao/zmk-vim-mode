package hyprland

import "testing"

func TestParseEvent(t *testing.T) {
	cases := []struct {
		line       string
		ok         bool
		class, ttl string
	}{
		{"activewindow>>com.mitchellh.ghostty,nvim ~/x.go", true, "com.mitchellh.ghostty", "nvim ~/x.go"},
		{"activewindow>>Code,main.go — zmk, vim, mode — Visual Studio Code", true, "Code", "main.go — zmk, vim, mode — Visual Studio Code"},
		{"activewindow>>,", true, "", ""},
		{"activewindowv2>>0x1234", false, "", ""},
		{"workspace>>3", false, "", ""},
		{"", false, "", ""},
	}
	for _, c := range cases {
		app, ok := ParseEvent(c.line)
		if ok != c.ok {
			t.Fatalf("%q: ok=%v want %v", c.line, ok, c.ok)
		}
		if !ok {
			continue
		}
		if !app.Known || app.Class != c.class || app.Title != c.ttl {
			t.Fatalf("%q: got %+v", c.line, app)
		}
	}
}
