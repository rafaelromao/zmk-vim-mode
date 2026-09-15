package modes

import "testing"

func TestRoundTrip(t *testing.T) {
	for m := Off; m <= LegacySilent; m++ {
		got := Decode(Encode(m, 0))
		if got != m {
			t.Fatalf("mode %v encoded to %#x, decoded as %v", m, Encode(m, 0), got)
		}
	}
}

func TestLocksAreLeftAlone(t *testing.T) {
	const numLock, capsLock = 0x01, 0x02
	leds := Encode(Insert, numLock|capsLock)
	if leds&numLock == 0 || leds&capsLock == 0 {
		t.Fatalf("lock bits were clobbered: %#x", leds)
	}
	if Decode(leds) != Insert {
		t.Fatalf("decoded %v, want insert", Decode(leds))
	}
}
