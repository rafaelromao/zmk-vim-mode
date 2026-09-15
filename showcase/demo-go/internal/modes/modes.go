// Package modes describes the eight editor states a host can put a ZMK
// keyboard in, and how they travel inside the HID LED indicator byte.
package modes

import "fmt"

// Mode is one of the eight 3-bit codes.
type Mode uint8

const (
	Off Mode = iota
	Normal
	Insert
	Visual
	Legacy
	Cmdline
	Raw
	LegacySilent
)

// The three indicator bits no operating system drives on its own.
const (
	ScrollLock = 0x04
	Compose    = 0x08
	Kana       = 0x10
)

var names = [...]string{"off", "normal", "insert", "visual", "legacy", "cmdline", "raw", "legacy-silent"}

// String returns the daemon's name for the mode.
func (m Mode) String() string {
	if int(m) < len(names) {
		return names[m]
	}
	return fmt.Sprintf("mode(%d)", uint8(m))
}

// Encode packs the mode into an LED report byte: Compose is bit 0 of the code,
// Kana bit 1 and Scroll Lock bit 2. Num Lock and Caps Lock are left untouched.
func Encode(m Mode, leds byte) byte {
	leds &^= Compose | Kana | ScrollLock
	if m&1 != 0 {
		leds |= Compose
	}
	if m&2 != 0 {
		leds |= Kana
	}
	if m&4 != 0 {
		leds |= ScrollLock
	}
	return leds
}

// Decode reads the mode back out of an LED report byte.
func Decode(leds byte) Mode {
	var m Mode
	if leds&Compose != 0 {
		m |= 1
	}
	if leds&Kana != 0 {
		m |= 2
	}
	if leds&ScrollLock != 0 {
		m |= 4
	}
	return m
}

// Layers names the keyboard layers a mode activates.
func (m Mode) Layers() []string {
	switch m {
	case Normal, Legacy, LegacySilent:
		return []string{"VIM_NORMAL"}
	case Insert:
		return []string{"VIM_INSERT"}
	case Visual:
		return []string{"VIM_NORMAL", "VIM_VISUAL"}
	case Cmdline:
		return []string{"VIM_CMDLINE"}
	default:
		return nil
	}
}
