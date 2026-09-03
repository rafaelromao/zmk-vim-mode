//go:build linux

package linux

import (
	"syscall"
	"unsafe"
)

// Linux input subsystem constants (linux/input.h, linux/input-event-codes.h).
// golang.org/x/sys is deliberately not used: everything needed is in syscall.
const (
	evSyn = 0x00
	evKey = 0x01
	evMsc = 0x04
	evLed = 0x11

	ledNumL    = 0x00
	ledCapsL   = 0x01
	ledScrollL = 0x02
	ledCompose = 0x03
	ledKana    = 0x04

	busUSB       = 0x03
	busBluetooth = 0x05

	iocWrite = 1
	iocRead  = 2
	iocTypeE = 'E'
)

// HID LED indicator bit positions in the ZMK output report (usage - 1).
const (
	hidNumLock    = 1 << 0
	hidCapsLock   = 1 << 1
	hidScrollLock = 1 << 2
	hidCompose    = 1 << 3
	hidKana       = 1 << 4
)

func ioc(dir, typ, nr, size uintptr) uintptr {
	return dir<<30 | size<<16 | typ<<8 | nr
}

func eviocgid() uintptr               { return ioc(iocRead, iocTypeE, 0x02, 8) }
func eviocgname(n uintptr) uintptr    { return ioc(iocRead, iocTypeE, 0x06, n) }
func eviocgled(n uintptr) uintptr     { return ioc(iocRead, iocTypeE, 0x19, n) }
func eviocgbit(ev, n uintptr) uintptr { return ioc(iocRead, iocTypeE, 0x20+ev, n) }
func eviocsmask() uintptr             { return ioc(iocWrite, iocTypeE, 0x93, 16) }

// inputID mirrors struct input_id.
type inputID struct {
	Bustype, Vendor, Product, Version uint16
}

// inputEvent mirrors struct input_event on 64-bit Linux (timeval = 2×int64).
type inputEvent struct {
	Sec   int64
	Usec  int64
	Type  uint16
	Code  uint16
	Value int32
}

const sizeofInputEvent = 24

// inputMask mirrors struct input_mask (EVIOCSMASK).
type inputMask struct {
	Type      uint32
	CodesSize uint32
	CodesPtr  uint64
}

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg))
	if e != 0 {
		return e
	}
	return nil
}

// codeToHIDBits maps the 3-bit mode code to LED indicator bits:
// b0 → Compose, b1 → Kana, b2 → Scroll Lock.
func codeToHIDBits(code uint8) uint8 {
	var b uint8
	if code&1 != 0 {
		b |= hidCompose
	}
	if code&2 != 0 {
		b |= hidKana
	}
	if code&4 != 0 {
		b |= hidScrollLock
	}
	return b
}

// kernelLEDsToHIDBits converts an EVIOCGLED bitmap (LED_* codes) to HID
// indicator bits for the two OS-owned indicators we must preserve.
func kernelLEDsToHIDBits(ledmap uint8) uint8 {
	var b uint8
	if ledmap&(1<<ledNumL) != 0 {
		b |= hidNumLock
	}
	if ledmap&(1<<ledCapsL) != 0 {
		b |= hidCapsLock
	}
	return b
}
