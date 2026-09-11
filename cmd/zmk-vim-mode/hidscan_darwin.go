//go:build darwin

package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	ledsdarwin "github.com/rafaelromao/zmk-vim-mode/internal/leds/darwin"
)

const hidScanSupported = true

// runHIDScan lists every HID device macOS knows, with the LED elements each
// exposes. It answers the first question when no keyboard is found: is the
// keyboard connected to this host at all, and does its firmware expose the
// three indicators the protocol needs?
func runHIDScan(args []string) error {
	fs := flag.NewFlagSet("hid-scan", flag.ContinueOnError)
	all := fs.Bool("all", false, "list every HID device, not just keyboards")
	if err := fs.Parse(args); err != nil {
		return err
	}
	entries := ledsdarwin.Scan()
	sort.Slice(entries, func(i, j int) bool { return entries[i].Product < entries[j].Product })
	shown := 0
	for _, e := range entries {
		keyboard := e.UsagePage == 0x01 && e.Usage == 0x06
		if !keyboard && !*all {
			continue
		}
		shown++
		leds := "none"
		if e.CodeLEDs() {
			leds = fmt.Sprintf("Compose+Kana+Scroll (report %d)", e.Compose)
		} else if e.NumLock >= 0 {
			leds = "Num/Caps only — firmware without CONFIG_ZMK_HID_INDICATORS=y"
		}
		name := e.Product
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Printf("%-32s %04x:%04x %-10s %s\n", name, e.VID, e.PID, e.Transport, leds)
	}
	if shown == 0 {
		fmt.Fprintln(os.Stderr, "no HID keyboards at all — is the keyboard connected to this host? (a ZMK keyboard talks to one BLE profile at a time)")
		if !*all {
			fmt.Fprintln(os.Stderr, "run with --all to list every HID device")
		}
	}
	return nil
}
