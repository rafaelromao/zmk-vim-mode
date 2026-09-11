//go:build darwin

package doctor

import focusdarwin "github.com/rafaelromao/zmk-vim-mode/internal/focus/darwin"

// platformChecks reports macOS-specific setup. Input Monitoring is not
// queryable, but the devices check above already shows whether it took.
func platformChecks() []check {
	if focusdarwin.AXTrusted() {
		return []check{{"accessibility", ok, "granted: window titles are readable, so tool windows are detected", ""}}
	}
	// Asking here rather than from the daemon: doctor is run by hand, so the
	// system dialog appears at a moment that makes sense. The grant applies to
	// the binary, hence to the daemon too.
	focusdarwin.PromptAXTrust()
	return []check{{"accessibility", warn,
		"not granted: window titles are invisible, so VSCode's tool windows keep the vim layers",
		"a dialog was just opened — or add ~/.local/bin/zmk-vim-mode under System Settings → Privacy & Security → Accessibility, then restart the agent"}}
}
