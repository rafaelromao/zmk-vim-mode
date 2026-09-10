// Package state holds the pure decision logic: which LED code the keyboard
// should show given the frontmost application, the connected editor clients
// and any manual override. It has no I/O.
package state

import "strings"

// Mode is the editor state the daemon reasons about.
type Mode uint8

const (
	Off     Mode = iota // no vim editor focused → all vim layers off
	Normal              // NORMAL (incl. operator-pending, terminal-normal)
	Insert              // INSERT (incl. Replace)
	Visual              // VISUAL / select
	Cmdline             // command line / prompts
	Raw                 // keys must pass through untouched: plugin UI buffers, terminal job, pending <leader>, tool windows
	Legacy              // vim-like app with no mode feed; keyboard infers locally (today's behaviour)
	// None is "no opinion": the client is connected but has nothing to say
	// about the mode right now -- a VSCode companion while the text editor has
	// focus, or any client whose timed report has expired. Never a decision.
	None
)

var modeNames = map[Mode]string{
	Off: "off", Normal: "normal", Insert: "insert", Visual: "visual",
	Cmdline: "cmdline", Raw: "raw", Legacy: "legacy", None: "none",
}

// Opinion reports whether m can drive a decision.
func (m Mode) Opinion() bool { return m != None }

// String returns the wire name of m.
func (m Mode) String() string {
	if s, ok := modeNames[m]; ok {
		return s
	}
	return "unknown"
}

// ParseMode maps a wire name to a Mode. An empty name is None (no opinion).
// Unknown names return (Normal, false): richer editor modes we do not model
// (IdeaVim's OP_PENDING*, SELECT_*) are treated as normal, and the caller may
// log the mismatch once.
func ParseMode(s string) (Mode, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "none":
		return None, true
	case "off":
		return Off, true
	case "normal", "n":
		return Normal, true
	case "insert", "i", "replace", "r":
		return Insert, true
	case "visual", "v", "select":
		return Visual, true
	case "cmdline", "c", "command", "command_line":
		return Cmdline, true
	case "raw":
		return Raw, true
	case "legacy":
		return Legacy, true
	}
	return Normal, false
}

// LED codes as decoded by the firmware module (b0 Compose, b1 Kana, b2 Scroll Lock).
const (
	CodeOff          uint8 = 0
	CodeNormal       uint8 = 1
	CodeInsert       uint8 = 2
	CodeVisual       uint8 = 3
	CodeLegacy       uint8 = 4 // transition into a legacy app: firmware runs &vim_mode_on (one ESC)
	CodeCmdline      uint8 = 5
	CodeRaw          uint8 = 6
	CodeLegacySilent uint8 = 7 // re-assert of Legacy after a clobber/reconnect: NORMAL layer, no bindings
)

// Code returns the LED code for m.
func (m Mode) Code() uint8 {
	switch m {
	case Normal:
		return CodeNormal
	case Insert:
		return CodeInsert
	case Visual:
		return CodeVisual
	case Cmdline:
		return CodeCmdline
	case Raw:
		return CodeRaw
	case Legacy:
		return CodeLegacy
	default:
		return CodeOff
	}
}

// SilentAlias returns the code to write when *re-asserting* an already
// applied state (device add, clobber echo): Legacy becomes LegacySilent so the
// firmware does not replay the ESC injection. All other codes are unchanged.
func SilentAlias(code uint8) uint8 {
	if code == CodeLegacy {
		return CodeLegacySilent
	}
	return code
}

// ModeName returns the wire name for a code (for status output).
func ModeName(code uint8) string {
	switch code {
	case CodeNormal:
		return Normal.String()
	case CodeInsert:
		return Insert.String()
	case CodeVisual:
		return Visual.String()
	case CodeCmdline:
		return Cmdline.String()
	case CodeRaw:
		return Raw.String()
	case CodeLegacy:
		return Legacy.String()
	case CodeLegacySilent:
		return "legacy_silent"
	default:
		return Off.String()
	}
}
