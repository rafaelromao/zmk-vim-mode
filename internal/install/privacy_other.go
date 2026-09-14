//go:build !darwin

package install

import "io"

// openPrivacyPanes exists only on macOS; other platforms handle device
// permissions through udev, which the installer writes itself.
func openPrivacyPanes(io.Writer) {}
