//go:build linux

// Package linux implements the LED backend for Linux: the mode code is written
// as a HID output report through /dev/hidrawN (so the kernel's own LED cache
// stays zero and compositor "write 0" storms are dropped by input_get_disposition),
// while the paired /dev/input/eventN is read for EV_LED echoes that reveal a
// real clobber (the kernel sent its own LED report), and watched for hotplug.
package linux

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/leds"
)

// Filter restricts which keyboards are driven. Zero values mean "any".
type Filter struct {
	VID, PID uint16
	// RequireCodeLEDs keeps only devices declaring Compose+Kana+Scroll Lock
	// LEDs (ZMK with CONFIG_ZMK_HID_INDICATORS does; Apple/most boards do not).
	RequireCodeLEDs bool
	// NameSubstring, when set, must appear in the device name (case-insensitive).
	NameSubstring string
}

// DefaultFilter drives every keyboard exposing the three carrier LEDs.
func DefaultFilter() Filter { return Filter{RequireCodeLEDs: true} }

// ReportID is the HID report ID of the LED output report (ZMK: 1).
const ReportID = 0x01

type dev struct {
	info   leds.Device
	ev     *evdevDev
	hidraw *os.File // nil when not available/writable
	hidDir string
}

// Backend is the Linux hidraw+evdev backend.
type Backend struct {
	log    *slog.Logger
	filter Filter

	mu     sync.Mutex
	devs   map[leds.DeviceID]*dev
	events chan<- leds.Event
	ctx    context.Context

	InputDir    string // /dev/input
	DevDir      string // /dev
	SysInput    string // /sys/class/input
	SysHidraw   string // /sys/class/hidraw
	RescanEvery time.Duration
}

// New creates the backend.
func New(log *slog.Logger, f Filter) *Backend {
	if log == nil {
		log = slog.Default()
	}
	return &Backend{
		log: log, filter: f, devs: map[leds.DeviceID]*dev{},
		InputDir: "/dev/input", DevDir: "/dev", SysInput: "/sys/class/input", SysHidraw: "/sys/class/hidraw",
		RescanEvery: 10 * time.Second,
	}
}

// Start scans once, then watches /dev/input and /dev via inotify (plus a slow
// periodic rescan as a safety net) until ctx is done.
func (b *Backend) Start(ctx context.Context, events chan<- leds.Event) error {
	b.mu.Lock()
	b.events = events
	b.ctx = ctx
	b.mu.Unlock()
	b.rescan()
	go b.watch(ctx)
	go func() {
		<-ctx.Done()
		b.mu.Lock()
		for id, d := range b.devs {
			b.closeDev(d)
			delete(b.devs, id)
		}
		b.mu.Unlock()
	}()
	return nil
}

// Devices lists known keyboards (writable or not, for doctor/status).
func (b *Backend) Devices() []leds.Device {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]leds.Device, 0, len(b.devs))
	for _, d := range b.devs {
		out = append(out, d.info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Write sends the code to one device.
func (b *Backend) Write(id leds.DeviceID, code uint8) error {
	b.mu.Lock()
	d, ok := b.devs[id]
	b.mu.Unlock()
	if !ok {
		return fmt.Errorf("device %s not present", id)
	}
	kernel, err := d.ev.ledState()
	if err != nil {
		kernel = 0
	}
	bits := codeToHIDBits(code) | kernelLEDsToHIDBits(kernel)
	if d.hidraw != nil {
		_, err := d.hidraw.Write([]byte{ReportID, bits})
		if err == nil {
			return nil
		}
		b.log.Debug("hidraw write failed, trying evdev", "dev", id, "err", err)
	}
	// Fallback: evdev EV_LED write (compositor may clobber it; the echo path re-asserts).
	return d.ev.writeLEDs(map[uint16]int32{
		ledScrollL: int32(code >> 2 & 1),
		ledCompose: int32(code & 1),
		ledKana:    int32(code >> 1 & 1),
	})
}

// rescan diffs the current /dev/input contents against known devices.
func (b *Backend) rescan() {
	entries, err := os.ReadDir(b.InputDir)
	if err != nil {
		b.log.Debug("read input dir", "err", err)
		return
	}
	hidToRaw := b.hidrawIndex()
	seen := map[leds.DeviceID]bool{}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "event") {
			continue
		}
		path := filepath.Join(b.InputDir, e.Name())
		hidDir := b.hidDirOf(e.Name())
		id := leds.DeviceID(e.Name())
		if hidDir != "" {
			id = leds.DeviceID(filepath.Base(hidDir))
		}
		seen[id] = true
		b.mu.Lock()
		existing, known := b.devs[id]
		b.mu.Unlock()
		if known {
			// Retry hidraw open if it was not writable before (udev ACL may have landed).
			if existing.hidraw == nil {
				if raw, ok := hidToRaw[hidDir]; ok {
					if f, err := openHidraw(raw); err == nil {
						b.mu.Lock()
						existing.hidraw = f
						existing.info.Writable = true
						existing.info.Note = ""
						b.mu.Unlock()
						b.emit(leds.Event{Kind: leds.Added, Dev: id})
					}
				}
			}
			continue
		}
		ev, err := openEvdev(path)
		if err != nil {
			if !errors.Is(err, errNotKeyboard) {
				b.log.Debug("probe failed", "path", path, "err", err)
			}
			continue
		}
		if !b.matches(ev) {
			ev.Close()
			continue
		}
		d := &dev{ev: ev, hidDir: hidDir, info: leds.Device{
			ID: id, VID: ev.id.Vendor, PID: ev.id.Product, Product: ev.name, Transport: ev.transport(),
		}}
		if raw, ok := hidToRaw[hidDir]; ok {
			if f, err := openHidraw(raw); err == nil {
				d.hidraw = f
				d.info.Writable = true
			} else {
				d.info.Note = fmt.Sprintf("%s not writable (%v); udev rule missing?", raw, err)
				d.info.Writable = ev.rdwr
			}
		} else {
			d.info.Note = "no hidraw node paired; using evdev writes"
			d.info.Writable = ev.rdwr
		}
		b.mu.Lock()
		b.devs[id] = d
		ctx := b.ctx
		b.mu.Unlock()
		b.log.Info("keyboard found", "id", id, "product", ev.name, "transport", d.info.Transport,
			"vid", fmt.Sprintf("%04x", ev.id.Vendor), "pid", fmt.Sprintf("%04x", ev.id.Product), "writable", d.info.Writable)
		if ctx != nil {
			go b.echoLoop(ctx, id, ev)
		}
		b.emit(leds.Event{Kind: leds.Added, Dev: id})
	}
	// Removed devices.
	b.mu.Lock()
	var gone []leds.DeviceID
	for id, d := range b.devs {
		if !seen[id] {
			b.closeDev(d)
			delete(b.devs, id)
			gone = append(gone, id)
		}
	}
	b.mu.Unlock()
	for _, id := range gone {
		b.log.Info("keyboard removed", "id", id)
		b.emit(leds.Event{Kind: leds.Removed, Dev: id})
	}
}

func (b *Backend) matches(ev *evdevDev) bool {
	if b.filter.VID != 0 && ev.id.Vendor != b.filter.VID {
		return false
	}
	if b.filter.PID != 0 && ev.id.Product != b.filter.PID {
		return false
	}
	if b.filter.RequireCodeLEDs && !ev.hasCodeLEDs() {
		return false
	}
	if s := b.filter.NameSubstring; s != "" && !strings.Contains(strings.ToLower(ev.name), strings.ToLower(s)) {
		return false
	}
	return true
}

func (b *Backend) closeDev(d *dev) {
	if d.ev != nil {
		d.ev.Close()
	}
	if d.hidraw != nil {
		d.hidraw.Close()
	}
}

func (b *Backend) emit(ev leds.Event) {
	b.mu.Lock()
	ch := b.events
	ctx := b.ctx
	b.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- ev:
	case <-ctx.Done():
	case <-time.After(time.Second):
		b.log.Warn("event channel stalled", "kind", ev.Kind, "dev", ev.Dev)
	}
}

// hidDirOf resolves /sys/class/input/eventN/device/device to the HID device dir.
func (b *Backend) hidDirOf(eventName string) string {
	p, err := filepath.EvalSymlinks(filepath.Join(b.SysInput, eventName, "device", "device"))
	if err != nil {
		return ""
	}
	// HID device dirs look like 0005:1D50:615E.0004; anything else is not HID.
	base := filepath.Base(p)
	if len(base) < 14 || base[4] != ':' || base[9] != ':' {
		return ""
	}
	return p
}

// hidrawIndex maps HID device dir → /dev/hidrawN.
func (b *Backend) hidrawIndex() map[string]string {
	out := map[string]string{}
	entries, err := os.ReadDir(b.SysHidraw)
	if err != nil {
		return out
	}
	for _, e := range entries {
		p, err := filepath.EvalSymlinks(filepath.Join(b.SysHidraw, e.Name(), "device"))
		if err != nil {
			continue
		}
		out[p] = filepath.Join(b.DevDir, e.Name())
	}
	return out
}

func openHidraw(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

// echoLoop reads the evdev node and reports EV_LED changes as Echo events.
func (b *Backend) echoLoop(ctx context.Context, id leds.DeviceID, ev *evdevDev) {
	buf := make([]byte, 64*sizeofInputEvent)
	for {
		evs, err := ev.readEvents(buf)
		if err != nil {
			return // closed on removal/shutdown
		}
		if ctx.Err() != nil {
			return
		}
		for _, e := range evs {
			if e.Type == evLed {
				b.log.Debug("led echo", "dev", id, "led", e.Code, "value", e.Value)
				b.emit(leds.Event{Kind: leds.Echo, Dev: id})
				break
			}
		}
	}
}

// watch uses inotify on /dev/input and /dev (hidraw nodes) with a debounce,
// plus a slow periodic rescan.
func (b *Backend) watch(ctx context.Context) {
	ifd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		b.log.Warn("inotify unavailable; falling back to periodic rescan", "err", err)
		b.periodic(ctx)
		return
	}
	f := os.NewFile(uintptr(ifd), "inotify")
	defer f.Close()
	mask := uint32(syscall.IN_CREATE | syscall.IN_ATTRIB | syscall.IN_DELETE)
	if _, err := syscall.InotifyAddWatch(ifd, b.InputDir, mask); err != nil {
		b.log.Warn("inotify watch failed", "dir", b.InputDir, "err", err)
	}
	if _, err := syscall.InotifyAddWatch(ifd, b.DevDir, mask); err != nil {
		b.log.Warn("inotify watch failed", "dir", b.DevDir, "err", err)
	}
	go func() {
		<-ctx.Done()
		f.Close()
	}()
	kick := make(chan struct{}, 1)
	go func() {
		buf := make([]byte, 64*1024)
		for {
			n, err := f.Read(buf)
			if err != nil {
				return
			}
			relevant := false
			for off := 0; off+syscall.SizeofInotifyEvent <= n; {
				raw := buf[off : off+syscall.SizeofInotifyEvent]
				nameLen := int(uint32(raw[12]) | uint32(raw[13])<<8 | uint32(raw[14])<<16 | uint32(raw[15])<<24)
				end := off + syscall.SizeofInotifyEvent + nameLen
				if end > n {
					break
				}
				name := strings.TrimRight(string(buf[off+syscall.SizeofInotifyEvent:end]), "\x00")
				if strings.HasPrefix(name, "event") || strings.HasPrefix(name, "hidraw") {
					relevant = true
				}
				off = end
			}
			if relevant {
				select {
				case kick <- struct{}{}:
				default:
				}
			}
		}
	}()
	var debounce *time.Timer
	ticker := time.NewTicker(b.RescanEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-kick:
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(150*time.Millisecond, b.rescan)
		case <-ticker.C:
			b.rescan()
		}
	}
}

func (b *Backend) periodic(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.rescan()
		}
	}
}
