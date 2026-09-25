// Package indicator turns the daemon's status into what a status bar shows:
// the macOS menu bar item, the Omarchy bar widget, a Waybar module. The bars
// only draw it, so the mode's wording lives here once instead of once per bar.
package indicator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rafaelromao/zmk-vim-mode/internal/proto"
)

// View is one status line for a bar, in the shape Waybar's custom modules
// read with return-type json. An empty Text means vim mode is off, which bars
// take as "hide the indicator".
type View struct {
	Text    string `json:"text"`
	Tooltip string `json:"tooltip"`
	// Class is the mode, or "error" when the daemon did not answer, for bars
	// that style by class.
	Class string `json:"class"`
}

// labels is the daemon's own vocabulary. "legacy" means a vim-like editor is
// focused but nothing reports its mode, so the keyboard infers it by watching
// keys. There is no "replace": the daemon reports it as insert.
var labels = map[string]string{
	"normal":  "NORMAL",
	"insert":  "INSERT",
	"visual":  "VISUAL",
	"cmdline": "CMDLINE",
	"legacy":  "VIM",
	"raw":     "RAW",
}

// FromReply is what a bar shows for the daemon's answer to a status request,
// whatever that answer was: a bar reads only what it is given, so every way
// the request can fail has to come out as a view too.
func FromReply(reply proto.Msg, err error) View {
	switch {
	case err != nil:
		return FromError(err)
	case reply.T == proto.TError:
		return FromError(fmt.Errorf("%s: %s", reply.Err, reply.Message))
	case reply.Status == nil:
		return FromError(errors.New("malformed status reply"))
	}
	return FromStatus(reply.Status)
}

// FromStatus is what a bar shows for the daemon's state.
func FromStatus(st *proto.Status) View {
	switch st.Mode {
	case "", "off", "none":
		return View{Tooltip: "Vim mode off", Class: "off"}
	}
	label, ok := labels[st.Mode]
	if !ok {
		label = strings.ToUpper(st.Mode)
	}
	tip := fmt.Sprintf("Vim mode: %s (code %d)", st.Mode, st.Code)
	if st.Reason != "" {
		tip += "\n" + st.Reason
	}
	if o := st.Override; o != nil && o.Mode != "" {
		tip += "\noverride: " + o.Mode
		if o.Sticky {
			tip += " (sticky)"
		}
	}
	return View{Text: label, Tooltip: tip, Class: st.Mode}
}

// FromError is what a bar shows when the daemon did not answer: a question
// mark rather than nothing, so a daemon that is down is not mistaken for vim
// mode being off.
func FromError(err error) View {
	return View{Text: "VIM ?", Tooltip: "zmk-vim-mode unavailable: " + err.Error(), Class: "error"}
}
