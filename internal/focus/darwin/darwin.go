//go:build darwin

// Package darwin reports the frontmost application on macOS.
//
// It polls NSWorkspace.frontmostApplication rather than observing
// NSWorkspaceDidActivateApplicationNotification: those notifications are
// delivered only through the process' main run loop, which a Go daemon does
// not run, while the property is a plain Launch Services query that works from
// any thread. Ten polls a second cost nothing measurable and put the worst
// case latency at one poll. No window titles: reading them needs the Screen
// Recording or Accessibility permission, so the title heuristics are off here.
package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
#include <string.h>

// zvm_frontmost writes the frontmost app's bundle identifier (or localized
// name) into buf and returns its pid, or 0 when there is none.
static int zvm_frontmost(char *buf, int n) {
	@autoreleasepool {
		NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
		if (!app) { buf[0] = 0; return 0; }
		NSString *id = app.bundleIdentifier ?: app.localizedName ?: @"";
		strncpy(buf, id.UTF8String, n - 1);
		buf[n - 1] = 0;
		return (int)app.processIdentifier;
	}
}
*/
import "C"

import (
	"context"
	"log/slog"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
)

// Watcher implements focus.Watcher.
type Watcher struct {
	log      *slog.Logger
	Interval time.Duration
}

// New creates a Watcher.
func New(log *slog.Logger) *Watcher {
	if log == nil {
		log = slog.Default()
	}
	return &Watcher{log: log, Interval: 100 * time.Millisecond}
}

// Frontmost returns the current frontmost application.
func Frontmost() focus.App {
	var buf [512]C.char
	pid := int(C.zvm_frontmost(&buf[0], 512))
	return focus.App{Known: true, Class: C.GoString(&buf[0]), PID: pid}
}

// Run emits the frontmost app whenever it changes, until ctx is done.
func (w *Watcher) Run(ctx context.Context, emit func(focus.App)) error {
	var last focus.App
	first := true
	t := time.NewTicker(w.Interval)
	defer t.Stop()
	for {
		cur := Frontmost()
		if first || !cur.SameIdentity(last) {
			emit(cur)
			last, first = cur, false
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}
