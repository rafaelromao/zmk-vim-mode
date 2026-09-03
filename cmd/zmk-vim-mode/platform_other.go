//go:build !linux

package main

import (
	"log/slog"
	"runtime"

	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
	"github.com/rafaelromao/zmk-vim-mode/internal/focus/noop"
	"github.com/rafaelromao/zmk-vim-mode/internal/leds"
	"github.com/rafaelromao/zmk-vim-mode/internal/leds/null"
)

const platformName = runtime.GOOS

// newBackend returns a logging-only backend until the macOS IOKit backend lands (plan phase 6).
func newBackend(log *slog.Logger, _ deviceFilter) leds.Backend {
	log.Warn("no LED backend for this platform yet; using the virtual (log-only) backend", "os", runtime.GOOS)
	return null.New(log)
}

func newFocusWatcher(log *slog.Logger) focus.Watcher {
	log.Warn("no focus backend for this platform yet; decisions trust editor clients", "os", runtime.GOOS)
	return noop.Watcher{}
}
