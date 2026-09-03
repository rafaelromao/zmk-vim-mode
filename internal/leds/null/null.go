// Package null is an LED backend that only logs writes. It lets the daemon run
// on platforms without a real backend yet (macOS today) so the socket protocol,
// the Neovim plugin and the decision logic can be exercised end to end.
package null

import (
	"context"
	"log/slog"

	"github.com/rafaelromao/zmk-vim-mode/internal/leds"
)

// Backend implements leds.Backend with one virtual device.
type Backend struct {
	log  *slog.Logger
	Last uint8
	Seen bool
}

// New creates the backend.
func New(log *slog.Logger) *Backend {
	if log == nil {
		log = slog.Default()
	}
	return &Backend{log: log}
}

const id leds.DeviceID = "null"

// Start announces the virtual device.
func (b *Backend) Start(ctx context.Context, events chan<- leds.Event) error {
	select {
	case events <- leds.Event{Kind: leds.Added, Dev: id}:
	case <-ctx.Done():
	}
	return nil
}

// Devices returns the virtual device.
func (b *Backend) Devices() []leds.Device {
	return []leds.Device{{ID: id, Product: "virtual (no LED backend on this platform)", Transport: "none", Writable: true}}
}

// Write logs the code.
func (b *Backend) Write(dev leds.DeviceID, code uint8) error {
	b.Last, b.Seen = code, true
	b.log.Info("virtual LED write", "code", code)
	return nil
}
