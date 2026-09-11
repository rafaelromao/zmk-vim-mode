//go:build linux

package main

import (
	"log/slog"

	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
	"github.com/rafaelromao/zmk-vim-mode/internal/focus/hyprland"
	"github.com/rafaelromao/zmk-vim-mode/internal/focus/noop"
	"github.com/rafaelromao/zmk-vim-mode/internal/leds"
	ledslinux "github.com/rafaelromao/zmk-vim-mode/internal/leds/linux"
)

const platformName = "linux"

// noDevicesHint is printed when no keyboard was found.
const noDevicesHint = `  - is CONFIG_ZMK_HID_INDICATORS=y in the central/dongle .conf and the firmware flashed?
  - is the keyboard connected to this host, and the udev rule installed? (zmk-vim-mode install --udev)`

func newBackend(log *slog.Logger, f deviceFilter) leds.Backend {
	return ledslinux.New(log, ledslinux.Filter{VID: f.vid, PID: f.pid, RequireCodeLEDs: !f.anyKeyboard, NameSubstring: f.name})
}

func newFocusWatcher(log *slog.Logger) focus.Watcher {
	h := hyprland.New(log)
	if h.Available() {
		return h
	}
	log.Warn("no Hyprland instance found; running without a focus backend (decisions trust editor clients)")
	// Keep trying Hyprland in the background: it re-discovers on each retry.
	return h
}

var _ focus.Watcher = noop.Watcher{}
