//go:build darwin

package daemon

import focusdarwin "github.com/rafaelromao/zmk-vim-mode/internal/focus/darwin"

// axTrusted reports whether *this* process may use the Accessibility API.
// It has to be answered by the daemon: macOS attributes such a request to the
// responsible process, so a CLI run from a terminal is judged on the
// terminal's permissions, not the daemon's.
func axTrusted() *bool {
	t := focusdarwin.AXTrusted()
	return &t
}
