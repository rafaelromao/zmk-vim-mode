package proto

import (
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	cases := []Msg{
		{V: 1, T: THello, Client: "nvim", PID: 42, Mode: "normal", Focused: Bool(true), Nested: false, Tmux: true, Plugin: "0.1.0"},
		{T: TMode, Mode: "insert", TTLMs: 400},
		{T: TFocus, Focused: Bool(false), Mode: "insert"},
		{T: TBye},
		{V: 1, T: TWelcome, Daemon: "0.1.0", Code: U8(1)},
		{T: TResync},
		{T: TError, Err: ErrUnsupportedVersion, Message: "daemon speaks v1"},
		{V: 1, T: TSet, Mode: "legacy", TTLMs: 30000, Sticky: true},
		{T: TOK, Code: U8(4), Mode: "legacy", Reason: "override"},
	}
	for _, in := range cases {
		b, err := Encode(in)
		if err != nil {
			t.Fatalf("encode %+v: %v", in, err)
		}
		if !strings.HasSuffix(string(b), "\n") || strings.Count(string(b), "\n") != 1 {
			t.Fatalf("encoded line must end with exactly one newline: %q", b)
		}
		out, err := Decode(b)
		if err != nil {
			t.Fatalf("decode %q: %v", b, err)
		}
		if out.T != in.T || out.Mode != in.Mode || out.Client != in.Client || out.PID != in.PID ||
			out.TTLMs != in.TTLMs || out.Sticky != in.Sticky || out.Err != in.Err || out.V != in.V {
			t.Fatalf("round trip mismatch:\n in=%+v\nout=%+v", in, out)
		}
		if (in.Focused == nil) != (out.Focused == nil) || (in.Focused != nil && *in.Focused != *out.Focused) {
			t.Fatalf("focused mismatch: %+v vs %+v", in, out)
		}
		if (in.Code == nil) != (out.Code == nil) || (in.Code != nil && *in.Code != *out.Code) {
			t.Fatalf("code mismatch: %+v vs %+v", in, out)
		}
	}
}

func TestDecodeToleratesUnknownFieldsAndWhitespace(t *testing.T) {
	m, err := Decode([]byte(`{"t":"mode","mode":"visual","future_field":{"x":1}}` + "\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m.T != TMode || m.Mode != "visual" {
		t.Fatalf("unexpected %+v", m)
	}
}

func TestDecodeErrors(t *testing.T) {
	for _, bad := range []string{"", "\n", "{}", `{"mode":"x"}`, "not json", strings.Repeat("a", MaxLine+1)} {
		if _, err := Decode([]byte(bad)); err == nil {
			t.Fatalf("expected error for %q", bad[:min(len(bad), 20)])
		}
	}
}

func TestEncodeRequiresType(t *testing.T) {
	if _, err := Encode(Msg{}); err == nil {
		t.Fatal("expected error")
	}
}
