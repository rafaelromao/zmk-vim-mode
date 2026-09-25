package indicator

import (
	"errors"
	"testing"

	"github.com/rafaelromao/zmk-vim-mode/internal/proto"
)

func TestFromStatus(t *testing.T) {
	cases := []struct {
		name string
		st   proto.Status
		want View
	}{
		{"off hides", proto.Status{Mode: "off"}, View{Tooltip: "Vim mode off", Class: "off"}},
		{"none hides", proto.Status{Mode: "none"}, View{Tooltip: "Vim mode off", Class: "off"}},
		{"empty hides", proto.Status{}, View{Tooltip: "Vim mode off", Class: "off"}},
		{"normal", proto.Status{Mode: "normal", Code: 1},
			View{Text: "NORMAL", Tooltip: "Vim mode: normal (code 1)", Class: "normal"}},
		{"reason on its own line", proto.Status{Mode: "insert", Code: 2, Reason: "nvim client reports insert"},
			View{Text: "INSERT", Tooltip: "Vim mode: insert (code 2)\nnvim client reports insert", Class: "insert"}},
		{"legacy reads VIM", proto.Status{Mode: "legacy", Code: 6},
			View{Text: "VIM", Tooltip: "Vim mode: legacy (code 6)", Class: "legacy"}},
		{"sticky override", proto.Status{Mode: "raw", Code: 5, Override: &proto.OverrideInfo{Mode: "raw", Sticky: true}},
			View{Text: "RAW", Tooltip: "Vim mode: raw (code 5)\noverride: raw (sticky)", Class: "raw"}},
		{"override with a TTL", proto.Status{Mode: "visual", Code: 3, Override: &proto.OverrideInfo{Mode: "visual", LeftMs: 5000}},
			View{Text: "VISUAL", Tooltip: "Vim mode: visual (code 3)\noverride: visual", Class: "visual"}},
		{"cmdline", proto.Status{Mode: "cmdline", Code: 4},
			View{Text: "CMDLINE", Tooltip: "Vim mode: cmdline (code 4)", Class: "cmdline"}},
		{"a mode this build does not know", proto.Status{Mode: "select", Code: 9},
			View{Text: "SELECT", Tooltip: "Vim mode: select (code 9)", Class: "select"}},
	}
	for _, c := range cases {
		if got := FromStatus(&c.st); got != c.want {
			t.Errorf("%s:\n got  %+v\n want %+v", c.name, got, c.want)
		}
	}
}

// Every way a status request can fail must still come out as something a bar
// can draw, and the same thing: the menu bar and the Omarchy widget would
// otherwise disagree about a daemon that is down.
func TestFromReply(t *testing.T) {
	st := &proto.Status{Mode: "normal", Code: 1}
	cases := []struct {
		name  string
		reply proto.Msg
		err   error
		want  View
	}{
		{"status", proto.Msg{T: proto.TStatus, Status: st}, nil,
			View{Text: "NORMAL", Tooltip: "Vim mode: normal (code 1)", Class: "normal"}},
		{"daemon down", proto.Msg{}, errors.New("daemon not reachable at x"),
			View{Text: "VIM ?", Tooltip: "zmk-vim-mode unavailable: daemon not reachable at x", Class: "error"}},
		{"error reply", proto.Msg{T: proto.TError, Err: proto.ErrUnsupportedVersion, Message: "want v1"}, nil,
			View{Text: "VIM ?", Tooltip: "zmk-vim-mode unavailable: unsupported_version: want v1", Class: "error"}},
		{"reply without a status", proto.Msg{T: proto.TStatus}, nil,
			View{Text: "VIM ?", Tooltip: "zmk-vim-mode unavailable: malformed status reply", Class: "error"}},
	}
	for _, c := range cases {
		if got := FromReply(c.reply, c.err); got != c.want {
			t.Errorf("%s:\n got  %+v\n want %+v", c.name, got, c.want)
		}
	}
}
