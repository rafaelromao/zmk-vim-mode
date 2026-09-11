//go:build darwin

package doctor

import focusdarwin "github.com/rafaelromao/zmk-vim-mode/internal/focus/darwin"

// platformChecks reports macOS-specific setup. daemonAX is what the daemon
// itself answered, which is the only answer that counts: macOS attributes an
// Accessibility request to the responsible process, so this CLI run from a
// terminal is judged on that terminal's permissions instead of the daemon's.
// Input Monitoring is not queryable at all; the devices check covers it.
func platformChecks(daemonAX *bool) []check {
	if daemonAX != nil {
		if *daemonAX {
			return []check{{"accessibility", ok, "granted to the daemon: window titles are readable, so tool windows are detected", ""}}
		}
		return []check{{"accessibility", warn,
			"not granted to the daemon: window titles are invisible, so VSCode's tool windows keep the vim layers",
			"System Settings → Privacy & Security → Accessibility → add ~/.local/bin/zmk-vim-mode, then: launchctl kickstart -k gui/$UID/dev.rafaelromao.zmk-vim-mode"}}
	}
	// No daemon to ask: fall back to this process, and prompt while a human is
	// here to answer.
	if focusdarwin.AXTrusted() {
		return []check{{"accessibility", info, "granted to this CLI (the daemon is not running, so it could not be asked)", ""}}
	}
	focusdarwin.PromptAXTrust()
	return []check{{"accessibility", warn, "not granted to this CLI, and the daemon is not running",
		"a dialog was just opened; the entry that matters is ~/.local/bin/zmk-vim-mode"}}
}
