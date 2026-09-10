package hyprland

import (
	"testing"

	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
)

func TestTrackerTrigger(t *testing.T) {
	tr := &tracker{}
	if !tr.trigger("activewindow>>Code,main.go []") {
		t.Fatal("activewindow must trigger a query")
	}
	if tr.trigger("activewindowv2>>5581f0a1b2c0") {
		t.Fatal("activewindowv2 only records the focused address")
	}
	if tr.addr != "5581f0a1b2c0" {
		t.Fatalf("addr = %q", tr.addr)
	}
	if tr.trigger("windowtitle>>deadbeef") {
		t.Fatal("a background window's title must not trigger")
	}
	if !tr.trigger("windowtitle>>5581f0a1b2c0") {
		t.Fatal("the focused window's title must trigger")
	}
	if !tr.trigger("windowtitlev2>>5581f0a1b2c0,main.go — zmk [Terminal]") {
		t.Fatal("windowtitlev2 for the focused window must trigger")
	}
	if tr.trigger("windowtitle>>5581f0a1b2c0") {
		t.Fatal("v1 title lines are duplicates once v2 has been seen")
	}
	if tr.trigger("windowtitlev2>>deadbeef,other") {
		t.Fatal("v2 title of a background window must not trigger")
	}
	if tr.trigger("workspace>>3") || tr.trigger("") {
		t.Fatal("unrelated lines must not trigger")
	}
	fresh := &tracker{}
	if !fresh.trigger("windowtitlev2>>abc,def") {
		t.Fatal("with the focused address unknown, any title change is worth a query")
	}
}

func TestTrackerNote(t *testing.T) {
	tr := &tracker{}
	a := focus.App{Known: true, Class: "Code", Title: "main.go []", PID: 7}
	if !tr.note(a, "0x5581F0A1B2C0") {
		t.Fatal("first result must emit")
	}
	if tr.addr != "5581f0a1b2c0" {
		t.Fatalf("addr from JSON must be normalised: %q", tr.addr)
	}
	if tr.note(a, "0x5581F0A1B2C0") {
		t.Fatal("identical result must not emit")
	}
	a.Title = "main.go [Terminal]"
	if !tr.note(a, "") {
		t.Fatal("a title change must emit")
	}
}

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
