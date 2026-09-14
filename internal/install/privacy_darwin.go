//go:build darwin

package install

import (
	"fmt"
	"io"
	"os/exec"
)

// Deep links into the Privacy & Security panes. These anchors are what System
// Settings itself uses; an unknown one lands on the pane's first list rather
// than failing, so a future macOS rename degrades to "close enough".
const (
	inputMonitoringPane = "x-apple.systempreferences:com.apple.preference.security?Privacy_ListenEvent"
	accessibilityPane   = "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"
)

// openPrivacyPanes brings up the two lists the user has to edit by hand. The
// grants themselves cannot be scripted: TCC's database is SIP-protected, and
// writing it is exactly what macOS is designed to prevent.
func openPrivacyPanes(w io.Writer) {
	// One at a time, Input Monitoring last: System Settings shows the pane it
	// was asked for most recently, and Input Monitoring is the grant without
	// which nothing works at all.
	for _, p := range []struct{ name, url string }{
		{"Accessibility", accessibilityPane},
		{"Input Monitoring", inputMonitoringPane},
	} {
		if err := exec.Command("open", p.url).Run(); err != nil {
			fmt.Fprintf(w, "  could not open %s (%v); System Settings → Privacy & Security → %s\n", p.name, err, p.name)
			continue
		}
		fmt.Fprintf(w, "  opened System Settings → Privacy & Security → %s\n", p.name)
	}
}
