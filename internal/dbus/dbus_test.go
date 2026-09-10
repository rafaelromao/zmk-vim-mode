package dbus

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestSplitSig(t *testing.T) {
	cases := []struct{ in, first, rest string }{
		{"s", "s", ""},
		{"ssv", "s", "sv"},
		{"a{ss}u", "a{ss}", "u"},
		{"a(yv)", "a(yv)", ""},
		{"(so)s", "(so)", "s"},
		{"aa{sv}i", "aa{sv}", "i"},
		{"siiva{sv}", "s", "iiva{sv}"},
	}
	for _, c := range cases {
		first, rest, err := splitSig(c.in)
		if err != nil || first != c.first || rest != c.rest {
			t.Errorf("%q: got (%q, %q, %v)", c.in, first, rest, err)
		}
	}
	if _, _, err := splitSig("(s"); err == nil {
		t.Error("unterminated struct must fail")
	}
}

func TestStringEncoding(t *testing.T) {
	var e encoder
	if err := e.value("s", "ab"); err != nil {
		t.Fatal(err)
	}
	want := []byte{2, 0, 0, 0, 'a', 'b', 0}
	if !bytes.Equal(e.buf, want) {
		t.Fatalf("got % x want % x", e.buf, want)
	}
	// A second string is 4-aligned after the NUL.
	if err := e.value("s", "c"); err != nil {
		t.Fatal(err)
	}
	want = append(want, 0, 1, 0, 0, 0, 'c', 0)
	if !bytes.Equal(e.buf, want) {
		t.Fatalf("got % x want % x", e.buf, want)
	}
}

func TestArrayLengthExcludesPadding(t *testing.T) {
	// a(yv): the array length counts from the first element (8-aligned), not
	// from the byte after the length field.
	var e encoder
	if err := e.value("a(yv)", []any{[]any{byte(1), Variant{"s", "x"}}}); err != nil {
		t.Fatal(err)
	}
	n := binary.LittleEndian.Uint32(e.buf[0:4])
	// element starts 8-aligned at 8: y(1) + sig "s"(3) = 4, then u32 len + "x\0" = 6 → 10 bytes
	if n != 10 || len(e.buf) != 8+10 {
		t.Fatalf("len=%d buf=% x", n, e.buf)
	}
}

func TestRoundTrip(t *testing.T) {
	sig := "siiva{sv}(so)a{ss}buxtd"
	args := []any{
		"focused", int32(1), int32(0), Variant{"i", int32(0)},
		map[string]any{"sender": Variant{"s", "org.x"}},
		[]any{":1.5", objectPath("/org/a11y/atspi/accessible/7")},
		map[string]string{"tag": "textarea", "class": "inputarea monaco-mouse-cursor-text"},
		true, uint32(4242), int64(-9), uint64(9), 2.5,
	}
	body, err := marshalBody(sig, args)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[byte]any{
		fieldPath: objectPath("/org/a11y/atspi/accessible/7"), fieldInterface: "org.a11y.atspi.Event.Object",
		fieldMember: "StateChanged", fieldSender: ":1.5", fieldSignature: signature(sig),
	}
	msg, err := encodeMessage(TypeSignal, 0, 77, fields, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg)%8 != len(body)%8 {
		t.Fatalf("header not 8-padded: %d", len(msg)-len(body))
	}
	m, err := readMessage(bytes.NewReader(msg))
	if err != nil {
		t.Fatal(err)
	}
	if m.Type != TypeSignal || m.Serial != 77 || m.Signature != sig {
		t.Fatalf("header: %+v", m)
	}
	if p, _ := String(m.Fields[fieldPath]); p != "/org/a11y/atspi/accessible/7" {
		t.Fatalf("path: %v", m.Fields[fieldPath])
	}
	if s, _ := String(m.Body[0]); s != "focused" {
		t.Fatalf("detail: %v", m.Body[0])
	}
	if m.Body[1] != int32(1) || m.Body[2] != int32(0) {
		t.Fatalf("details: %v %v", m.Body[1], m.Body[2])
	}
	if v, ok := m.Body[3].(Variant); !ok || v.Sig != "i" || v.Value != int32(0) {
		t.Fatalf("variant: %#v", m.Body[3])
	}
	props := m.Body[4].(map[string]any)
	if s, _ := String(props["sender"]); s != "org.x" {
		t.Fatalf("a{sv}: %#v", props)
	}
	st, ok := Struct(m.Body[5])
	if !ok || len(st) != 2 {
		t.Fatalf("struct: %#v", m.Body[5])
	}
	if o, _ := String(st[1]); o != "/org/a11y/atspi/accessible/7" {
		t.Fatalf("struct path: %#v", st[1])
	}
	attrs := StringMap(m.Body[6])
	if !reflect.DeepEqual(attrs, map[string]string{"tag": "textarea", "class": "inputarea monaco-mouse-cursor-text"}) {
		t.Fatalf("a{ss}: %#v", attrs)
	}
	if m.Body[7] != true || m.Body[8] != uint32(4242) || m.Body[9] != int64(-9) || m.Body[10] != uint64(9) || m.Body[11] != 2.5 {
		t.Fatalf("scalars: %v", m.Body[7:])
	}
}

func TestMethodCallShape(t *testing.T) {
	body, err := marshalBody("ssv", []any{"org.a11y.Status", "IsEnabled", Variant{"b", true}})
	if err != nil {
		t.Fatal(err)
	}
	msg, err := encodeMessage(TypeMethodCall, 0, 1, map[byte]any{
		fieldPath: objectPath("/org/a11y/bus"), fieldDestination: "org.a11y.Bus",
		fieldInterface: "org.freedesktop.DBus.Properties", fieldMember: "Set", fieldSignature: signature("ssv"),
	}, body)
	if err != nil {
		t.Fatal(err)
	}
	if msg[0] != 'l' || msg[1] != TypeMethodCall || msg[3] != 1 {
		t.Fatalf("fixed header: % x", msg[:4])
	}
	if binary.LittleEndian.Uint32(msg[4:8]) != uint32(len(body)) {
		t.Fatal("body length")
	}
	m, err := readMessage(bytes.NewReader(msg))
	if err != nil {
		t.Fatal(err)
	}
	if d, _ := String(m.Fields[fieldDestination]); d != "org.a11y.Bus" {
		t.Fatalf("dest: %v", m.Fields[fieldDestination])
	}
	if v := m.Body[2].(Variant); v.Sig != "b" || v.Value != true {
		t.Fatalf("variant b: %#v", v)
	}
}

func TestEmptyBodyAndUnixAddress(t *testing.T) {
	if b, err := marshalBody("", nil); err != nil || b != nil {
		t.Fatalf("empty body: %v %v", b, err)
	}
	if _, err := marshalBody("", []any{"extra"}); err == nil {
		t.Fatal("args without signature must fail")
	}
	if p, _ := unixSocket("unix:path=/run/user/1000/at-spi/bus_0,guid=abc"); p != "/run/user/1000/at-spi/bus_0" {
		t.Fatalf("path: %q", p)
	}
	if p, _ := unixSocket("unix:abstract=/tmp/dbus-XYZ"); p != "@/tmp/dbus-XYZ" {
		t.Fatalf("abstract: %q", p)
	}
	if _, err := unixSocket("tcp:host=x"); err == nil {
		t.Fatal("tcp must be rejected")
	}
}

func TestBigEndianRead(t *testing.T) {
	// A big-endian METHOD_RETURN with reply serial 3 and body "u" = 7.
	var b []byte
	b = append(b, 'B', TypeMethodReturn, 0, 1)
	b = binary.BigEndian.AppendUint32(b, 4) // body length
	b = binary.BigEndian.AppendUint32(b, 9) // serial
	// fields: (y=5, v(u 3)) and (y=8, v(g "u"))
	var f []byte
	f = append(f, 5, 1, 'u', 0)             // code, sig "u"
	f = binary.BigEndian.AppendUint32(f, 3) // value (4-aligned at 4)
	f = append(f, 8, 1, 'g', 0, 1, 'u', 0)  // code 8 at 8, sig "g", value sig "u"
	b = binary.BigEndian.AppendUint32(b, uint32(len(f)))
	b = append(b, f...)
	for len(b)%8 != 0 {
		b = append(b, 0)
	}
	b = binary.BigEndian.AppendUint32(b, 7)
	m, err := readMessage(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if m.Fields[fieldReplySerial] != uint32(3) || m.Body[0] != uint32(7) {
		t.Fatalf("%+v", m)
	}
}
