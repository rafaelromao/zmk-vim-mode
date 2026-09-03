// Package noop is a focus watcher for platforms/sessions without a supported
// backend. It reports "unknown" once so the decision logic fails open and
// trusts the editor clients.
package noop

import (
	"context"

	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
)

// Watcher implements focus.Watcher.
type Watcher struct{}

// Run emits an unknown App and blocks until ctx is done.
func (Watcher) Run(ctx context.Context, emit func(focus.App)) error {
	emit(focus.App{})
	<-ctx.Done()
	return nil
}
