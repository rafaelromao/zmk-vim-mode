//go:build !darwin

package daemon

// axTrusted is macOS-only; elsewhere there is nothing to report.
func axTrusted() *bool { return nil }
