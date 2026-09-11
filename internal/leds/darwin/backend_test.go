//go:build darwin

package darwin

import (
	"context"
	"testing"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/leds"
)

func TestCodeToBits(t *testing.T) {
	cases := map[uint8]uint8{0: 0, 1: bitCompose, 2: bitKana, 3: bitCompose | bitKana, 4: bitScrollLock, 7: bitCompose | bitKana | bitScrollLock}
	for code, want := range cases {
		if got := codeToBits(code); got != want {
			t.Errorf("code %d: got %#x want %#x", code, got, want)
		}
	}
}

// The manager thread must start and stop cleanly even with no matching
// keyboard (the usual state on a build machine).
func TestStartStopWithoutDevices(t *testing.T) {
	b := New(nil, Filter{VID: 0x1d50, PID: 0x615e, RequireCodeLEDs: true})
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	events := make(chan leds.Event, 8)
	if err := b.Start(ctx, events); err != nil {
		t.Fatal(err)
	}
	<-ctx.Done()
	time.Sleep(50 * time.Millisecond)
	if err := b.Write("hid-none", 1); err == nil {
		t.Fatal("writing to an unknown device must fail")
	}
}
