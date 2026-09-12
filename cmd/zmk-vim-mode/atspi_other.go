//go:build !linux

package main

import (
	"errors"
	"log/slog"

	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
)

// newWidgetWatcher has nothing to offer outside Linux: AT-SPI2 is the Linux
// accessibility bus. Asking for it anyway is a mistake worth naming rather
// than a watcher that retries a bus that will never be there.
func newWidgetWatcher(log *slog.Logger, enabled bool) focus.WidgetWatcher {
	if enabled {
		log.Warn("--atspi is Linux-only (AT-SPI2 does not exist on this platform); ignoring it")
	}
	return nil
}

func runATSPIWatch([]string) error {
	return errors.New("atspi-watch is Linux-only: AT-SPI2 is the Linux accessibility bus")
}
