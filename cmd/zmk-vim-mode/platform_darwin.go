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

func newBackend(log *slog.Logger, f deviceFilter) leds.Backend {
	return ledsdarwin.New(log, ledsdarwin.Filter{VID: f.vid, PID: f.pid, RequireCodeLEDs: !f.anyKeyboard, NameSubstring: f.name})
}

func newFocusWatcher(log *slog.Logger) focus.Watcher {
	return focusdarwin.New(log)
}
