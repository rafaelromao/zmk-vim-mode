// Package dbus is a minimal D-Bus client: enough to talk to the session bus
// and the AT-SPI2 accessibility bus -- method calls, properties, match rules,
// signals. It exists because the daemon must hold one long-lived connection
// (AT-SPI applications only emit events to a listener that registered from a
// connection that is still open), and a full binding would be the project's
// first dependency. Little-endian is written; either endianness is read.
package dbus

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Message types.
const (
	TypeMethodCall   byte = 1
	TypeMethodReturn byte = 2
	TypeError        byte = 3
	TypeSignal       byte = 4
)

// Header field codes.
const (
	fieldPath        byte = 1
	fieldInterface   byte = 2
	fieldMember      byte = 3
	fieldErrorName   byte = 4
	fieldReplySerial byte = 5
	fieldDestination byte = 6
	fieldSender      byte = 7
	fieldSignature   byte = 8
)

// Variant is a D-Bus variant: a signature and the value it wraps.
type Variant struct {
	Sig   string
	Value any
}

// Signal is a received signal.
type Signal struct {
	Sender, Path, Interface, Member string
	Body                            []any
}

// Message is a decoded wire message.
type Message struct {
	Type      byte
	Serial    uint32
	Fields    map[byte]any
	Signature string
	Body      []any
}

// Error is a D-Bus error reply.
type Error struct {
	Name    string
	Message string
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Name + ": " + e.Message
	}
	return e.Name
}

// Conn is one bus connection.
type Conn struct {
	c       net.Conn
	wmu     sync.Mutex
	serial  uint32
	pmu     sync.Mutex
	pending map[uint32]chan *Message
	signals chan Signal
	done    chan struct{}
	err     error
	Name    string // unique name assigned by Hello
	Dropped uint64 // signals dropped because nobody read them in time
}

// SessionBusAddress returns $DBUS_SESSION_BUS_ADDRESS or the systemd default.
func SessionBusAddress() string {
	if a := os.Getenv("DBUS_SESSION_BUS_ADDRESS"); a != "" {
		return a
	}
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return "unix:path=" + filepath.Join(dir, "bus")
}

// Dial connects to a bus address (unix:path=... or unix:abstract=...;
// alternatives separated by ';'), authenticates as this uid and says Hello.
func Dial(ctx context.Context, address string) (*Conn, error) {
	var lastErr error
	for _, alt := range strings.Split(address, ";") {
		sock, err := unixSocket(alt)
		if err != nil {
			lastErr = err
			continue
		}
		var d net.Dialer
		nc, err := d.DialContext(ctx, "unix", sock)
		if err != nil {
			lastErr = err
			continue
		}
		c := &Conn{c: nc, pending: map[uint32]chan *Message{}, signals: make(chan Signal, 256), done: make(chan struct{})}
		if dl, ok := ctx.Deadline(); ok {
			_ = nc.SetDeadline(dl)
		}
		if err := c.auth(); err != nil {
			nc.Close()
			return nil, fmt.Errorf("dbus auth: %w", err)
		}
		_ = nc.SetDeadline(time.Time{})
		go c.readLoop()
		name, err := c.Call(ctx, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "Hello", "")
		if err != nil {
			c.Close()
			return nil, fmt.Errorf("dbus hello: %w", err)
		}
		if len(name) == 1 {
			c.Name, _ = name[0].(string)
		}
		return c, nil
	}
	if lastErr == nil {
		lastErr = errors.New("empty address")
	}
	return nil, lastErr
}

// unixSocket extracts the socket path from one unix: address.
func unixSocket(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if !strings.HasPrefix(addr, "unix:") {
		return "", fmt.Errorf("unsupported transport in %q", addr)
	}
	for _, kv := range strings.Split(addr[len("unix:"):], ",") {
		k, v, _ := strings.Cut(kv, "=")
		switch k {
		case "path":
			return v, nil
		case "abstract":
			return "@" + v, nil
		}
	}
	return "", fmt.Errorf("no path in %q", addr)
}

// auth runs SASL EXTERNAL with this process' uid, then BEGIN.
func (c *Conn) auth() error {
	if _, err := c.c.Write([]byte{0}); err != nil {
		return err
	}
	uid := fmt.Sprintf("%x", []byte(strconv.Itoa(os.Getuid())))
	if _, err := fmt.Fprintf(c.c, "AUTH EXTERNAL %s\r\n", uid); err != nil {
		return err
	}
	line, err := readLine(c.c)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, "OK ") {
		return fmt.Errorf("server said %q", line)
	}
	_, err = c.c.Write([]byte("BEGIN\r\n"))
	return err
}

func readLine(r io.Reader) (string, error) {
	var b []byte
	one := make([]byte, 1)
	for {
		if _, err := io.ReadFull(r, one); err != nil {
			return "", err
		}
		b = append(b, one[0])
		if len(b) >= 2 && b[len(b)-2] == '\r' && b[len(b)-1] == '\n' {
			return string(b[:len(b)-2]), nil
		}
		if len(b) > 4096 {
			return "", errors.New("auth line too long")
		}
	}
}

// Close shuts the connection down.
func (c *Conn) Close() error {
	return c.c.Close()
}

// Signals returns the channel of received signals matching the rules added
// with AddMatch. It is closed when the connection dies (see Err).
func (c *Conn) Signals() <-chan Signal { return c.signals }

// Err returns why the connection died, once Signals is closed.
func (c *Conn) Err() error {
	select {
	case <-c.done:
		return c.err
	default:
		return nil
	}
}

// Call invokes a method and returns the reply body. sig is the signature of
// args (may be empty).
func (c *Conn) Call(ctx context.Context, dest, path, iface, method, sig string, args ...any) ([]any, error) {
	body, err := marshalBody(sig, args)
	if err != nil {
		return nil, err
	}
	fields := map[byte]any{
		fieldPath:        objectPath(path),
		fieldDestination: dest,
		fieldInterface:   iface,
		fieldMember:      method,
	}
	if sig != "" {
		fields[fieldSignature] = signature(sig)
	}
	serial, ch := c.newPending()
	if err := c.write(TypeMethodCall, serial, fields, body); err != nil {
		c.dropPending(serial)
		return nil, err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	select {
	case m, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("connection closed: %w", c.Err())
		}
		if m.Type == TypeError {
			e := &Error{}
			e.Name, _ = m.Fields[fieldErrorName].(string)
			if len(m.Body) > 0 {
				e.Message, _ = m.Body[0].(string)
			}
			return nil, e
		}
		return m.Body, nil
	case <-ctx.Done():
		c.dropPending(serial)
		return nil, ctx.Err()
	}
}

// AddMatch subscribes to signals matching rule.
func (c *Conn) AddMatch(ctx context.Context, rule string) error {
	_, err := c.Call(ctx, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "AddMatch", "s", rule)
	return err
}

// GetProperty reads a property through org.freedesktop.DBus.Properties.
func (c *Conn) GetProperty(ctx context.Context, dest, path, iface, prop string) (Variant, error) {
	body, err := c.Call(ctx, dest, path, "org.freedesktop.DBus.Properties", "Get", "ss", iface, prop)
	if err != nil {
		return Variant{}, err
	}
	if len(body) != 1 {
		return Variant{}, fmt.Errorf("Get returned %d values", len(body))
	}
	v, ok := body[0].(Variant)
	if !ok {
		return Variant{}, fmt.Errorf("Get returned %T, not a variant", body[0])
	}
	return v, nil
}

// SetProperty writes a property through org.freedesktop.DBus.Properties.
func (c *Conn) SetProperty(ctx context.Context, dest, path, iface, prop string, v Variant) error {
	_, err := c.Call(ctx, dest, path, "org.freedesktop.DBus.Properties", "Set", "ssv", iface, prop, v)
	return err
}

// ConnectionPID asks the bus for the process id behind a connection name.
func (c *Conn) ConnectionPID(ctx context.Context, name string) (int, error) {
	body, err := c.Call(ctx, "org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "GetConnectionUnixProcessID", "s", name)
	if err != nil {
		return 0, err
	}
	if len(body) != 1 {
		return 0, fmt.Errorf("GetConnectionUnixProcessID returned %d values", len(body))
	}
	pid, ok := body[0].(uint32)
	if !ok {
		return 0, fmt.Errorf("pid is %T", body[0])
	}
	return int(pid), nil
}

func (c *Conn) newPending() (uint32, chan *Message) {
	c.pmu.Lock()
	defer c.pmu.Unlock()
	c.serial++
	if c.serial == 0 {
		c.serial = 1
	}
	ch := make(chan *Message, 1)
	c.pending[c.serial] = ch
	return c.serial, ch
}

func (c *Conn) dropPending(serial uint32) {
	c.pmu.Lock()
	delete(c.pending, serial)
	c.pmu.Unlock()
}

func (c *Conn) write(typ byte, serial uint32, fields map[byte]any, body []byte) error {
	msg, err := encodeMessage(typ, 0, serial, fields, body)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, err = c.c.Write(msg)
	return err
}

func (c *Conn) readLoop() {
	for {
		m, err := readMessage(c.c)
		if err != nil {
			c.fail(err)
			return
		}
		switch m.Type {
		case TypeMethodReturn, TypeError:
			rs, _ := m.Fields[fieldReplySerial].(uint32)
			c.pmu.Lock()
			ch, ok := c.pending[rs]
			delete(c.pending, rs)
			c.pmu.Unlock()
			if ok {
				ch <- m
			}
		case TypeSignal:
			s := Signal{Body: m.Body}
			s.Sender, _ = m.Fields[fieldSender].(string)
			s.Interface, _ = m.Fields[fieldInterface].(string)
			s.Member, _ = m.Fields[fieldMember].(string)
			if p, ok := m.Fields[fieldPath].(objectPath); ok {
				s.Path = string(p)
			}
			select {
			case c.signals <- s:
			default:
				c.Dropped++
			}
		}
	}
}

func (c *Conn) fail(err error) {
	c.pmu.Lock()
	c.err = err
	for k, ch := range c.pending {
		close(ch)
		delete(c.pending, k)
	}
	c.pmu.Unlock()
	close(c.signals)
	close(c.done)
}

// ---- wire format -----------------------------------------------------------

// objectPath and signature are the string-like types with their own codes.
type objectPath string
type signature string

func alignment(t byte) int {
	switch t {
	case 'y', 'g', 'v':
		return 1
	case 'n', 'q':
		return 2
	case 'b', 'i', 'u', 's', 'o', 'a':
		return 4
	case 'x', 't', 'd', '(', '{':
		return 8
	}
	return 1
}

// splitSig returns the first complete single type of sig and the rest.
func splitSig(sig string) (string, string, error) {
	if sig == "" {
		return "", "", errors.New("empty signature")
	}
	switch sig[0] {
	case 'a':
		inner, rest, err := splitSig(sig[1:])
		return "a" + inner, rest, err
	case '(', '{':
		close := byte(')')
		if sig[0] == '{' {
			close = '}'
		}
		depth := 0
		for i := 0; i < len(sig); i++ {
			switch sig[i] {
			case '(', '{':
				depth++
			case ')', '}':
				depth--
				if depth == 0 {
					if sig[i] != close {
						return "", "", fmt.Errorf("mismatched brackets in %q", sig)
					}
					return sig[:i+1], sig[i+1:], nil
				}
			}
		}
		return "", "", fmt.Errorf("unterminated container in %q", sig)
	}
	return sig[:1], sig[1:], nil
}

type encoder struct {
	buf []byte
}

func (e *encoder) pad(n int) {
	for len(e.buf)%n != 0 {
		e.buf = append(e.buf, 0)
	}
}

func (e *encoder) u32(v uint32) {
	e.pad(4)
	e.buf = binary.LittleEndian.AppendUint32(e.buf, v)
}

func (e *encoder) str(s string) {
	e.u32(uint32(len(s)))
	e.buf = append(e.buf, s...)
	e.buf = append(e.buf, 0)
}

func (e *encoder) sig(s string) {
	e.buf = append(e.buf, byte(len(s)))
	e.buf = append(e.buf, s...)
	e.buf = append(e.buf, 0)
}

// value encodes v as the single complete type sig.
func (e *encoder) value(sig string, v any) error {
	switch sig[0] {
	case 'y':
		b, ok := v.(byte)
		if !ok {
			return typeErr(sig, v)
		}
		e.buf = append(e.buf, b)
	case 'b':
		b, ok := v.(bool)
		if !ok {
			return typeErr(sig, v)
		}
		var u uint32
		if b {
			u = 1
		}
		e.u32(u)
	case 'n':
		n, ok := v.(int16)
		if !ok {
			return typeErr(sig, v)
		}
		e.pad(2)
		e.buf = binary.LittleEndian.AppendUint16(e.buf, uint16(n))
	case 'q':
		n, ok := v.(uint16)
		if !ok {
			return typeErr(sig, v)
		}
		e.pad(2)
		e.buf = binary.LittleEndian.AppendUint16(e.buf, n)
	case 'i':
		n, ok := v.(int32)
		if !ok {
			return typeErr(sig, v)
		}
		e.u32(uint32(n))
	case 'u':
		n, ok := v.(uint32)
		if !ok {
			return typeErr(sig, v)
		}
		e.u32(n)
	case 'x':
		n, ok := v.(int64)
		if !ok {
			return typeErr(sig, v)
		}
		e.pad(8)
		e.buf = binary.LittleEndian.AppendUint64(e.buf, uint64(n))
	case 't':
		n, ok := v.(uint64)
		if !ok {
			return typeErr(sig, v)
		}
		e.pad(8)
		e.buf = binary.LittleEndian.AppendUint64(e.buf, n)
	case 'd':
		f, ok := v.(float64)
		if !ok {
			return typeErr(sig, v)
		}
		e.pad(8)
		e.buf = binary.LittleEndian.AppendUint64(e.buf, math.Float64bits(f))
	case 's':
		s, ok := v.(string)
		if !ok {
			return typeErr(sig, v)
		}
		e.str(s)
	case 'o':
		switch s := v.(type) {
		case objectPath:
			e.str(string(s))
		case string:
			e.str(s)
		default:
			return typeErr(sig, v)
		}
	case 'g':
		switch s := v.(type) {
		case signature:
			e.sig(string(s))
		case string:
			e.sig(s)
		default:
			return typeErr(sig, v)
		}
	case 'v':
		vr, ok := v.(Variant)
		if !ok {
			return typeErr(sig, v)
		}
		e.sig(vr.Sig)
		return e.value(vr.Sig, vr.Value)
	case 'a':
		return e.array(sig, v)
	case '(':
		items, ok := v.([]any)
		if !ok {
			return typeErr(sig, v)
		}
		e.pad(8)
		rest := sig[1 : len(sig)-1]
		for _, it := range items {
			var first string
			var err error
			first, rest, err = splitSig(rest)
			if err != nil {
				return err
			}
			if err := e.value(first, it); err != nil {
				return err
			}
		}
		if rest != "" {
			return fmt.Errorf("struct %q: %d values do not fill it", sig, len(items))
		}
	default:
		return fmt.Errorf("cannot encode type %q", sig)
	}
	return nil
}

func (e *encoder) array(sig string, v any) error {
	elem := sig[1:]
	e.u32(0) // length placeholder
	lenAt := len(e.buf) - 4
	e.pad(alignment(elem[0]))
	start := len(e.buf)
	switch elem[0] {
	case '{':
		inner := elem[1 : len(elem)-1]
		ksig, vsig, err := splitSig(inner)
		if err != nil {
			return err
		}
		switch m := v.(type) {
		case map[string]string:
			for k, val := range m {
				e.pad(8)
				if err := e.value(ksig, k); err != nil {
					return err
				}
				if err := e.value(vsig, val); err != nil {
					return err
				}
			}
		case map[string]any:
			for k, val := range m {
				e.pad(8)
				if err := e.value(ksig, k); err != nil {
					return err
				}
				if err := e.value(vsig, val); err != nil {
					return err
				}
			}
		default:
			return typeErr(sig, v)
		}
	default:
		items, ok := v.([]any)
		if !ok {
			return typeErr(sig, v)
		}
		for _, it := range items {
			if err := e.value(elem, it); err != nil {
				return err
			}
		}
	}
	binary.LittleEndian.PutUint32(e.buf[lenAt:], uint32(len(e.buf)-start))
	return nil
}

func typeErr(sig string, v any) error {
	return fmt.Errorf("cannot encode %T as %q", v, sig)
}

// marshalBody encodes args as sig, relative to a body that starts 8-aligned.
func marshalBody(sig string, args []any) ([]byte, error) {
	if sig == "" {
		if len(args) != 0 {
			return nil, errors.New("arguments without a signature")
		}
		return nil, nil
	}
	var e encoder
	rest := sig
	for i, a := range args {
		if rest == "" {
			return nil, fmt.Errorf("signature %q exhausted at argument %d", sig, i)
		}
		var first string
		var err error
		first, rest, err = splitSig(rest)
		if err != nil {
			return nil, err
		}
		if err := e.value(first, a); err != nil {
			return nil, err
		}
	}
	if rest != "" {
		return nil, fmt.Errorf("signature %q longer than the %d arguments", sig, len(args))
	}
	return e.buf, nil
}

// encodeMessage builds a complete message.
func encodeMessage(typ, flags byte, serial uint32, fields map[byte]any, body []byte) ([]byte, error) {
	var e encoder
	e.buf = append(e.buf, 'l', typ, flags, 1)
	e.buf = binary.LittleEndian.AppendUint32(e.buf, uint32(len(body)))
	e.buf = binary.LittleEndian.AppendUint32(e.buf, serial)
	var items []any
	for _, code := range []byte{fieldPath, fieldInterface, fieldMember, fieldErrorName, fieldReplySerial, fieldDestination, fieldSender, fieldSignature} {
		v, ok := fields[code]
		if !ok {
			continue
		}
		var vr Variant
		switch x := v.(type) {
		case objectPath:
			vr = Variant{"o", x}
		case signature:
			vr = Variant{"g", x}
		case string:
			vr = Variant{"s", x}
		case uint32:
			vr = Variant{"u", x}
		default:
			return nil, fmt.Errorf("header field %d: %T", code, v)
		}
		items = append(items, []any{code, vr})
	}
	if err := e.value("a(yv)", items); err != nil {
		return nil, err
	}
	e.pad(8)
	return append(e.buf, body...), nil
}

type decoder struct {
	buf   []byte
	pos   int
	order binary.ByteOrder
}

func (d *decoder) align(n int) error {
	p := (d.pos + n - 1) / n * n
	if p > len(d.buf) {
		return io.ErrUnexpectedEOF
	}
	d.pos = p
	return nil
}

func (d *decoder) need(n int) ([]byte, error) {
	if d.pos+n > len(d.buf) {
		return nil, io.ErrUnexpectedEOF
	}
	b := d.buf[d.pos : d.pos+n]
	d.pos += n
	return b, nil
}

func (d *decoder) u32() (uint32, error) {
	if err := d.align(4); err != nil {
		return 0, err
	}
	b, err := d.need(4)
	if err != nil {
		return 0, err
	}
	return d.order.Uint32(b), nil
}

func (d *decoder) str() (string, error) {
	n, err := d.u32()
	if err != nil {
		return "", err
	}
	b, err := d.need(int(n) + 1)
	if err != nil {
		return "", err
	}
	return string(b[:n]), nil
}

func (d *decoder) sig() (string, error) {
	b, err := d.need(1)
	if err != nil {
		return "", err
	}
	s, err := d.need(int(b[0]) + 1)
	if err != nil {
		return "", err
	}
	return string(s[:b[0]]), nil
}

// value decodes one complete type sig.
func (d *decoder) value(sig string) (any, error) {
	switch sig[0] {
	case 'y':
		b, err := d.need(1)
		if err != nil {
			return nil, err
		}
		return b[0], nil
	case 'b':
		u, err := d.u32()
		return u != 0, err
	case 'n', 'q':
		if err := d.align(2); err != nil {
			return nil, err
		}
		b, err := d.need(2)
		if err != nil {
			return nil, err
		}
		if sig[0] == 'n' {
			return int16(d.order.Uint16(b)), nil
		}
		return d.order.Uint16(b), nil
	case 'i':
		u, err := d.u32()
		return int32(u), err
	case 'u':
		return d.u32()
	case 'x', 't', 'd':
		if err := d.align(8); err != nil {
			return nil, err
		}
		b, err := d.need(8)
		if err != nil {
			return nil, err
		}
		u := d.order.Uint64(b)
		switch sig[0] {
		case 'x':
			return int64(u), nil
		case 'd':
			return math.Float64frombits(u), nil
		}
		return u, nil
	case 's':
		return d.str()
	case 'o':
		s, err := d.str()
		return objectPath(s), err
	case 'g':
		s, err := d.sig()
		return signature(s), err
	case 'v':
		s, err := d.sig()
		if err != nil {
			return nil, err
		}
		if s == "" {
			return nil, errors.New("empty variant signature")
		}
		v, err := d.value(s)
		return Variant{s, v}, err
	case 'a':
		return d.array(sig)
	case '(':
		if err := d.align(8); err != nil {
			return nil, err
		}
		var out []any
		rest := sig[1 : len(sig)-1]
		for rest != "" {
			var first string
			var err error
			first, rest, err = splitSig(rest)
			if err != nil {
				return nil, err
			}
			v, err := d.value(first)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}
	return nil, fmt.Errorf("cannot decode type %q", sig)
}

func (d *decoder) array(sig string) (any, error) {
	n, err := d.u32()
	if err != nil {
		return nil, err
	}
	elem := sig[1:]
	if err := d.align(alignment(elem[0])); err != nil {
		return nil, err
	}
	end := d.pos + int(n)
	if end > len(d.buf) {
		return nil, io.ErrUnexpectedEOF
	}
	if elem[0] == '{' {
		inner := elem[1 : len(elem)-1]
		ksig, vsig, err := splitSig(inner)
		if err != nil {
			return nil, err
		}
		out := map[string]any{}
		for d.pos < end {
			if err := d.align(8); err != nil {
				return nil, err
			}
			k, err := d.value(ksig)
			if err != nil {
				return nil, err
			}
			v, err := d.value(vsig)
			if err != nil {
				return nil, err
			}
			out[fmt.Sprint(k)] = v
		}
		d.pos = end
		return out, nil
	}
	out := []any{}
	for d.pos < end {
		v, err := d.value(elem)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	d.pos = end
	return out, nil
}

// readMessage reads and decodes one message from r.
func readMessage(r io.Reader) (*Message, error) {
	head := make([]byte, 16)
	if _, err := io.ReadFull(r, head); err != nil {
		return nil, err
	}
	var order binary.ByteOrder = binary.LittleEndian
	switch head[0] {
	case 'l':
	case 'B':
		order = binary.BigEndian
	default:
		return nil, fmt.Errorf("bad endianness byte %q", head[0])
	}
	if head[3] != 1 {
		return nil, fmt.Errorf("unsupported protocol version %d", head[3])
	}
	bodyLen := order.Uint32(head[4:8])
	fieldsLen := order.Uint32(head[12:16])
	if fieldsLen > 1<<20 || bodyLen > 1<<26 {
		return nil, fmt.Errorf("oversized message (fields %d, body %d)", fieldsLen, bodyLen)
	}
	headerLen := 16 + int(fieldsLen)
	padded := (headerLen + 7) / 8 * 8
	buf := make([]byte, padded)
	copy(buf, head)
	if _, err := io.ReadFull(r, buf[16:]); err != nil {
		return nil, err
	}
	m := &Message{Type: head[1], Serial: order.Uint32(head[8:12]), Fields: map[byte]any{}}
	d := &decoder{buf: buf, pos: 12, order: order}
	raw, err := d.value("a(yv)")
	if err != nil {
		return nil, fmt.Errorf("header fields: %w", err)
	}
	for _, it := range raw.([]any) {
		pair := it.([]any)
		code, _ := pair[0].(byte)
		v, _ := pair[1].(Variant)
		m.Fields[code] = v.Value
	}
	if s, ok := m.Fields[fieldSignature].(signature); ok {
		m.Signature = string(s)
	}
	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	if m.Signature != "" {
		bd := &decoder{buf: body, order: order}
		rest := m.Signature
		for rest != "" {
			var first string
			first, rest, err = splitSig(rest)
			if err != nil {
				return nil, err
			}
			v, err := bd.value(first)
			if err != nil {
				return nil, fmt.Errorf("body (%s): %w", m.Signature, err)
			}
			m.Body = append(m.Body, v)
		}
	}
	return m, nil
}

// ---- helpers for callers -------------------------------------------------------

// String extracts a string from a decoded value (string, object path, signature).
func String(v any) (string, bool) {
	switch s := v.(type) {
	case string:
		return s, true
	case objectPath:
		return string(s), true
	case signature:
		return string(s), true
	case Variant:
		return String(s.Value)
	}
	return "", false
}

// StringMap extracts an a{ss} (or a{sv} of strings) as a plain map.
func StringMap(v any) map[string]string {
	out := map[string]string{}
	m, ok := v.(map[string]any)
	if !ok {
		return out
	}
	for k, val := range m {
		if s, ok := String(val); ok {
			out[k] = s
		}
	}
	return out
}

// Struct extracts a struct's members.
func Struct(v any) ([]any, bool) {
	if vr, ok := v.(Variant); ok {
		v = vr.Value
	}
	s, ok := v.([]any)
	return s, ok
}
