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
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
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

static int zvm_ax_trusted(void) { return AXIsProcessTrusted() ? 1 : 0; }

// zvm_ax_prompt asks the system to show the "grant Accessibility" dialog.
static int zvm_ax_prompt(void) {
	CFStringRef keys[] = { kAXTrustedCheckOptionPrompt };
	CFBooleanRef vals[] = { kCFBooleanTrue };
	CFDictionaryRef opts = CFDictionaryCreate(kCFAllocatorDefault, (const void **)keys, (const void **)vals, 1,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	int ok = AXIsProcessTrustedWithOptions(opts) ? 1 : 0;
	CFRelease(opts);
	return ok;
}

// zvm_window_title writes the title of pid's focused window into buf. Needs
// the Accessibility permission; without it the attribute query simply fails
// and the title stays empty.
static void zvm_window_title(int pid, char *buf, int n) {
	buf[0] = 0;
	AXUIElementRef app = AXUIElementCreateApplication((pid_t)pid);
	if (!app) return;
	CFTypeRef win = NULL;
	if (AXUIElementCopyAttributeValue(app, kAXFocusedWindowAttribute, &win) == kAXErrorSuccess && win) {
		CFTypeRef title = NULL;
		if (AXUIElementCopyAttributeValue((AXUIElementRef)win, kAXTitleAttribute, &title) == kAXErrorSuccess && title) {
			if (CFGetTypeID(title) == CFStringGetTypeID())
				CFStringGetCString((CFStringRef)title, buf, n, kCFStringEncodingUTF8);
			CFRelease(title);
		}
		CFRelease(win);
	}
	CFRelease(app);
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

// AXTrusted reports whether this process may use the Accessibility API, which
// is what makes window titles readable.
func AXTrusted() bool { return C.zvm_ax_trusted() != 0 }

// PromptAXTrust asks macOS to show the Accessibility permission dialog. It
// returns the current state; the grant itself only takes effect for a later
// run of the process.
func PromptAXTrust() bool { return C.zvm_ax_prompt() != 0 }

// Frontmost returns the current frontmost application. The title is filled in
// only when the Accessibility permission is granted.
func Frontmost(withTitle bool) focus.App {
	var buf [512]C.char
	pid := int(C.zvm_frontmost(&buf[0], 512))
	app := focus.App{Known: true, Class: C.GoString(&buf[0]), PID: pid}
	if withTitle && pid != 0 {
		var t [1024]C.char
		C.zvm_window_title(C.int(pid), &t[0], 1024)
		app.Title = C.GoString(&t[0])
	}
	return app
}

// Run emits the frontmost app whenever it changes, until ctx is done. Title
// changes count as changes: VSCode publishes its focused view there, which is
// how tool windows are detected.
func (w *Watcher) Run(ctx context.Context, emit func(focus.App)) error {
	titles := AXTrusted()
	if titles {
		w.log.Info("reading window titles through the Accessibility API")
	} else {
		w.log.Warn("no Accessibility permission: window titles are invisible, so VSCode's tool windows " +
			"cannot be detected (System Settings → Privacy & Security → Accessibility; `zmk-vim-mode doctor` asks for it)")
	}
	var last focus.App
	first := true
	t := time.NewTicker(w.Interval)
	defer t.Stop()
	for {
		// Re-check the grant now and then: it can be given while we run, and
		// it needs no restart to take effect.
		if !titles && first {
			titles = AXTrusted()
		}
		cur := Frontmost(titles)
		if first || !cur.SameIdentity(last) || cur.Title != last.Title {
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
