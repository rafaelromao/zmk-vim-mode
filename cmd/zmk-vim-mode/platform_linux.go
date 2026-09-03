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
