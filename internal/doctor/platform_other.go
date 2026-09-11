//go:build !darwin

package doctor

// platformChecks has nothing to add outside macOS.
func platformChecks(*bool) []check { return nil }
