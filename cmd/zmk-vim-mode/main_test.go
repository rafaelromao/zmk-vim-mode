package main

import (
	"testing"

	"github.com/rafaelromao/zmk-vim-mode/internal/proto"
)

func TestOverrideLabel(t *testing.T) {
	for _, tt := range []struct {
		o    proto.OverrideInfo
		want string
	}{
		{proto.OverrideInfo{Mode: "legacy", Manual: true}, "manual (editor tracking with legacy fallback)"},
		{proto.OverrideInfo{Mode: "legacy"}, "legacy"},
		{proto.OverrideInfo{Mode: "off", Sticky: true}, "off"},
	} {
		if got := overrideLabel(&tt.o); got != tt.want {
			t.Errorf("%+v: got %q, want %q", tt.o, got, tt.want)
		}
	}
}
