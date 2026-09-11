//go:build !darwin

package main

import "errors"

const hidScanSupported = false

// runHIDScan exists only on macOS; on Linux the device nodes and
// scripts/spike-linux.sh cover the same ground.
func runHIDScan([]string) error {
	return errors.New("hid-scan is macOS-only; on Linux use `zmk-vim-mode devices` and scripts/spike-linux.sh find")
}
