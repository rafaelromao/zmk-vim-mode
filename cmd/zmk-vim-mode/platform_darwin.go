//go:build darwin

package main

import (
	"log/slog"

	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
	focusdarwin "github.com/rafaelromao/zmk-vim-mode/internal/focus/darwin"
	"github.com/rafaelromao/zmk-vim-mode/internal/leds"
	ledsdarwin "github.com/rafaelromao/zmk-vim-mode/internal/leds/darwin"
)

const platformName = "darwin"

// noDevicesHint is printed when no keyboard was found. On macOS the usual
// cause is the keyboard talking to another host: ZMK serves one BLE profile
// at a time.
const noDevicesHint = `  - is the keyboard connected to this Mac? (switch its BLE profile) — check with: zmk-vim-mode hid-scan
  - is CONFIG_ZMK_HID_INDICATORS=y in the central/dongle .conf and the firmware flashed?
  - Input Monitoring is needed to *write*, not to find: a device seen but not writable says so.`

func newBackend(log *slog.Logger, f deviceFilter) leds.Backend {
	return ledsdarwin.New(log, ledsdarwin.Filter{VID: f.vid, PID: f.pid, RequireCodeLEDs: !f.anyKeyboard, NameSubstring: f.name})
}

func newFocusWatcher(log *slog.Logger) focus.Watcher {
	return focusdarwin.New(log)
}
