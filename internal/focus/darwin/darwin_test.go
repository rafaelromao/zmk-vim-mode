//go:build darwin

package darwin

import (
	"context"
	"testing"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
)

// The query must work from a plain process without a run loop; whether an
// app is frontmost depends on the session, so only the shape is checked.
func TestFrontmostShape(t *testing.T) {
	app := Frontmost()
	if !app.Known {
		t.Fatal("frontmost must always be known on macOS")
	}
	if app.PID != 0 && app.Class == "" {
		t.Fatalf("a running app should have a bundle id or name: %+v", app)
	}
}

func TestRunEmitsOnceThenStops(t *testing.T) {
	w := New(nil)
	w.Interval = 10 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	n := 0
	if err := w.Run(ctx, func(a focus.App) { n++ }); err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatal("initial frontmost must be emitted")
	}
}
