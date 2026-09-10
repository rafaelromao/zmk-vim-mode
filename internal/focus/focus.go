// Package focus defines the frontmost-application abstraction shared by the
// platform focus watchers (Hyprland today; macOS/X11 later) and the decision logic.
package focus

import "context"

// App describes the frontmost application/window as far as the platform can tell.
type App struct {
	// Known is false when the focus backend is down or has not reported yet.
	// The decision logic then fails open (trusts editor clients) instead of forcing OFF.
	Known bool
	// Class is the Wayland app_id / X11 WM_CLASS / macOS bundle identifier.
	Class string
	// Title is the window title; only used by the optional title heuristic and
	// never logged above debug level.
	Title string
	PID   int
}

// SameIdentity reports whether two App values refer to the same application
// instance (used to expire non-sticky manual overrides on focus change).
func (a App) SameIdentity(b App) bool {
	return a.Known == b.Known && a.Class == b.Class && a.PID == b.PID
}

// Watcher streams frontmost-app changes. Run blocks until ctx is done; it must
// call emit once with the current state as soon as it is known and then on
// every change. Implementations reconnect on their own and must not return
// early on transient errors.
type Watcher interface {
	Run(ctx context.Context, emit func(App)) error
}

// Widget says where keyboard focus is *inside* an application: in its text
// editor, or in some other control (a quick input, a tree, a terminal). It
// comes from the accessibility bus and is keyed by process id, which is what
// the window watcher also reports, so the two can be joined.
type Widget struct {
	PID    int
	Editor bool
	Detail string // for logs and status, never anything typed
}

// WidgetWatcher streams Widget facts for every application that exposes them.
type WidgetWatcher interface {
	Run(ctx context.Context, emit func(Widget)) error
}
