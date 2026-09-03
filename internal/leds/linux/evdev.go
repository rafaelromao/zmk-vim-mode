//go:build linux

package linux

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// evdevDev is an open /dev/input/eventN node.
type evdevDev struct {
	path    string
	f       *os.File // non-blocking, registered with Go's poller (Read/Close are safe across goroutines)
	id      inputID
	name    string
	ledBits uint32 // EV_LED capability bitmap
	rdwr    bool
}

// openEvdev opens and probes an event node. It returns an error for nodes that
// are not keyboards with LED capability.
func openEvdev(path string) (*evdevDev, error) {
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	rdwr := true
	if err != nil {
		if !errors.Is(err, syscall.EACCES) && !errors.Is(err, syscall.EPERM) {
			return nil, err
		}
		fd, err = syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
		if err != nil {
			return nil, err
		}
		rdwr = false
	}
	d := &evdevDev{path: path, f: os.NewFile(uintptr(fd), path), rdwr: rdwr}
	if err := d.probe(); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

func (d *evdevDev) control(fn func(fd uintptr) error) error {
	rc, err := d.f.SyscallConn()
	if err != nil {
		return err
	}
	var inner error
	if err := rc.Control(func(fd uintptr) { inner = fn(fd) }); err != nil {
		return err
	}
	return inner
}

func (d *evdevDev) probe() error {
	return d.control(func(fd uintptr) error {
		if err := ioctl(fd, eviocgid(), unsafe.Pointer(&d.id)); err != nil {
			return fmt.Errorf("EVIOCGID: %w", err)
		}
		var name [256]byte
		if err := ioctl(fd, eviocgname(uintptr(len(name))), unsafe.Pointer(&name[0])); err == nil {
			d.name = strings.TrimRight(string(name[:]), "\x00")
		}
		var types [8]byte // EV_MAX/8 rounded up
		if err := ioctl(fd, eviocgbit(0, uintptr(len(types))), unsafe.Pointer(&types[0])); err != nil {
			return fmt.Errorf("EVIOCGBIT(0): %w", err)
		}
		hasKey := types[evKey/8]&(1<<(evKey%8)) != 0
		hasLed := types[evLed/8]&(1<<(evLed%8)) != 0
		if !hasKey || !hasLed {
			return errNotKeyboard
		}
		var leds [8]byte
		if err := ioctl(fd, eviocgbit(evLed, uintptr(len(leds))), unsafe.Pointer(&leds[0])); err != nil {
			return fmt.Errorf("EVIOCGBIT(EV_LED): %w", err)
		}
		d.ledBits = binary.LittleEndian.Uint32(leds[:4])
		// Privacy/CPU: never receive key or scancode events on this fd.
		for _, t := range []uint32{evKey, evMsc} {
			m := inputMask{Type: t, CodesSize: 0, CodesPtr: 0}
			_ = ioctl(fd, eviocsmask(), unsafe.Pointer(&m)) // best effort (kernel ≥ 4.4)
		}
		return nil
	})
}

var errNotKeyboard = errors.New("not a keyboard with LEDs")

// hasLED reports whether the device declares the given LED_* code.
func (d *evdevDev) hasLED(code uint) bool { return d.ledBits&(1<<code) != 0 }

// hasCodeLEDs reports whether the device exposes the three carrier LEDs.
func (d *evdevDev) hasCodeLEDs() bool {
	return d.hasLED(ledCompose) && d.hasLED(ledKana) && d.hasLED(ledScrollL)
}

// ledState returns the kernel's current LED bitmap (LED_* codes).
func (d *evdevDev) ledState() (uint8, error) {
	var out uint8
	err := d.control(func(fd uintptr) error {
		var buf [8]byte
		if err := ioctl(fd, eviocgled(uintptr(len(buf))), unsafe.Pointer(&buf[0])); err != nil {
			return err
		}
		out = buf[0]
		return nil
	})
	return out, err
}

// writeLEDs writes EV_LED events (fallback path when hidraw is unavailable).
// value bits: LED_* codes.
func (d *evdevDev) writeLEDs(set map[uint16]int32) error {
	if !d.rdwr {
		return errors.New("evdev node not writable")
	}
	buf := make([]byte, 0, (len(set)+1)*sizeofInputEvent)
	put := func(t, c uint16, v int32) {
		var ev [sizeofInputEvent]byte
		binary.LittleEndian.PutUint16(ev[16:], t)
		binary.LittleEndian.PutUint16(ev[18:], c)
		binary.LittleEndian.PutUint32(ev[20:], uint32(v))
		buf = append(buf, ev[:]...)
	}
	for c, v := range set {
		put(evLed, c, v)
	}
	put(evSyn, 0, 0)
	_, err := d.f.Write(buf)
	return err
}

// readEvents blocks until at least one event is available and returns them.
// It returns an error when the file is closed.
func (d *evdevDev) readEvents(buf []byte) ([]inputEvent, error) {
	n, err := d.f.Read(buf)
	if err != nil {
		return nil, err
	}
	evs := make([]inputEvent, 0, n/sizeofInputEvent)
	for off := 0; off+sizeofInputEvent <= n; off += sizeofInputEvent {
		b := buf[off : off+sizeofInputEvent]
		evs = append(evs, inputEvent{
			Sec:   int64(binary.LittleEndian.Uint64(b[0:8])),
			Usec:  int64(binary.LittleEndian.Uint64(b[8:16])),
			Type:  binary.LittleEndian.Uint16(b[16:18]),
			Code:  binary.LittleEndian.Uint16(b[18:20]),
			Value: int32(binary.LittleEndian.Uint32(b[20:24])),
		})
	}
	return evs, nil
}

// Close releases the node; a concurrent readEvents returns an error.
func (d *evdevDev) Close() error { return d.f.Close() }

func (d *evdevDev) transport() string {
	switch d.id.Bustype {
	case busUSB:
		return "usb"
	case busBluetooth:
		return "bluetooth"
	default:
		return fmt.Sprintf("bus-0x%02x", d.id.Bustype)
	}
}
