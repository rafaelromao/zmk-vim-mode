//go:build linux

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
	"github.com/rafaelromao/zmk-vim-mode/internal/focus/atspi"
)

// newWidgetWatcher returns the accessibility-bus watcher when --atspi is on.
// It is plain Go over unix sockets, so it builds everywhere; it only finds a
// bus on a Linux desktop.
func newWidgetWatcher(log *slog.Logger, enabled bool) focus.WidgetWatcher {
	if !enabled {
		return nil
	}
	return atspi.New(log)
}

// runATSPIWatch prints every focus event the accessibility bus reports, with
// the classifier's verdict, until interrupted. It is how the widget rules get
// tuned against a real VSCode.
func runATSPIWatch(args []string) error {
	fs := flag.NewFlagSet("atspi-watch", flag.ContinueOnError)
	level := fs.String("log-level", "info", "debug|info|warn|error")
	if err := fs.Parse(args); err != nil {
		return err
	}
	log, err := newLogger(*level)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Fprintln(os.Stderr, "watching the accessibility bus; click around in VSCode, Ctrl-C to stop")
	return atspi.New(log).Watch(ctx, os.Stdout)
}
