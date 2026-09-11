//go:build darwin

// Package darwin drives the keyboard's LED indicators through IOKit HID.
//
// macOS never writes the Compose, Kana or Scroll Lock indicators on its own,
// so there is no clobber to watch for: a code is written once and re-asserted
// on device arrival and on wake. Opening a keyboard's HID device needs the
// Input Monitoring permission (System Settings → Privacy & Security); a
// refused open is reported as a non-writable device with that hint.
package darwin

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>
#include <IOKit/IOMessage.h>
#include <IOKit/hid/IOHIDManager.h>
#include <IOKit/hid/IOHIDDevice.h>
#include <IOKit/hid/IOHIDElement.h>
#include <IOKit/hid/IOHIDValue.h>
#include <IOKit/hid/IOHIDKeys.h>
#include <IOKit/pwr_mgt/IOPMLib.h>

extern void zvmDeviceMatched(uintptr_t handle, IOHIDDeviceRef dev);
extern void zvmDeviceRemoved(uintptr_t handle, IOHIDDeviceRef dev);
extern void zvmSystemWake(uintptr_t handle);

static CFRunLoopRef zvm_loop;
static io_connect_t zvm_power_root;

static void zvm_matched_cb(void *ctx, IOReturn r, void *sender, IOHIDDeviceRef dev) {
	zvmDeviceMatched((uintptr_t)ctx, dev);
}
static void zvm_removed_cb(void *ctx, IOReturn r, void *sender, IOHIDDeviceRef dev) {
	zvmDeviceRemoved((uintptr_t)ctx, dev);
}
static void zvm_power_cb(void *ctx, io_service_t service, natural_t type, void *arg) {
	switch (type) {
	case kIOMessageCanSystemSleep:
	case kIOMessageSystemWillSleep:
		IOAllowPowerChange(zvm_power_root, (long)arg);
		break;
	case kIOMessageSystemHasPoweredOn:
		zvmSystemWake((uintptr_t)ctx);
		break;
	}
}

static void zvm_dict_set_int(CFMutableDictionaryRef d, CFStringRef key, int v) {
	CFNumberRef n = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &v);
	CFDictionarySetValue(d, key, n);
	CFRelease(n);
}

// zvm_run creates the HID manager on the calling thread's run loop and runs it
// until zvm_stop. vid/pid 0 match any keyboard.
static void zvm_run(uintptr_t handle, int vid, int pid) {
	IOHIDManagerRef mgr = IOHIDManagerCreate(kCFAllocatorDefault, kIOHIDOptionsTypeNone);
	CFMutableDictionaryRef match = CFDictionaryCreateMutable(kCFAllocatorDefault, 0,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	if (vid) zvm_dict_set_int(match, CFSTR(kIOHIDVendorIDKey), vid);
	if (pid) zvm_dict_set_int(match, CFSTR(kIOHIDProductIDKey), pid);
	if (!vid && !pid) {
		zvm_dict_set_int(match, CFSTR(kIOHIDDeviceUsagePageKey), 0x01);
		zvm_dict_set_int(match, CFSTR(kIOHIDDeviceUsageKey), 0x06);
	}
	IOHIDManagerSetDeviceMatching(mgr, match);
	CFRelease(match);
	IOHIDManagerRegisterDeviceMatchingCallback(mgr, zvm_matched_cb, (void *)handle);
	IOHIDManagerRegisterDeviceRemovalCallback(mgr, zvm_removed_cb, (void *)handle);
	zvm_loop = CFRunLoopGetCurrent();
	IOHIDManagerScheduleWithRunLoop(mgr, zvm_loop, kCFRunLoopDefaultMode);
	IOHIDManagerOpen(mgr, kIOHIDOptionsTypeNone);

	IONotificationPortRef port = NULL;
	io_object_t notifier = 0;
	zvm_power_root = IORegisterForSystemPower((void *)handle, &port, zvm_power_cb, &notifier);
	if (port) CFRunLoopAddSource(zvm_loop, IONotificationPortGetRunLoopSource(port), kCFRunLoopDefaultMode);

	CFRunLoopRun();

	IOHIDManagerClose(mgr, kIOHIDOptionsTypeNone);
	CFRelease(mgr);
}

static void zvm_stop(void) {
	if (zvm_loop) CFRunLoopStop(zvm_loop);
}

static int zvm_prop_int(IOHIDDeviceRef dev, CFStringRef key) {
	CFTypeRef v = IOHIDDeviceGetProperty(dev, key);
	int out = 0;
	if (v && CFGetTypeID(v) == CFNumberGetTypeID()) CFNumberGetValue((CFNumberRef)v, kCFNumberIntType, &out);
	return out;
}

static void zvm_prop_str(IOHIDDeviceRef dev, CFStringRef key, char *buf, int n) {
	buf[0] = 0;
	CFTypeRef v = IOHIDDeviceGetProperty(dev, key);
	if (v && CFGetTypeID(v) == CFStringGetTypeID()) CFStringGetCString((CFStringRef)v, buf, n, kCFStringEncodingUTF8);
}

static uint64_t zvm_entry_id(IOHIDDeviceRef dev) {
	uint64_t id = 0;
	io_service_t svc = IOHIDDeviceGetService(dev);
	if (svc) IORegistryEntryGetRegistryEntryID(svc, &id);
	return id;
}

// zvm_led_element finds the output element for one LED usage (page 0x08) and
// returns its report id, or -1 when the device has no such LED.
static int zvm_led_element(IOHIDDeviceRef dev, int usage) {
	CFMutableDictionaryRef m = CFDictionaryCreateMutable(kCFAllocatorDefault, 0,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	zvm_dict_set_int(m, CFSTR(kIOHIDElementUsagePageKey), 0x08);
	zvm_dict_set_int(m, CFSTR(kIOHIDElementUsageKey), usage);
	CFArrayRef els = IOHIDDeviceCopyMatchingElements(dev, m, kIOHIDOptionsTypeNone);
	CFRelease(m);
	int report = -1;
	if (els) {
		for (CFIndex i = 0; i < CFArrayGetCount(els); i++) {
			IOHIDElementRef e = (IOHIDElementRef)CFArrayGetValueAtIndex(els, i);
			if (IOHIDElementGetType(e) == kIOHIDElementTypeOutput) {
				report = (int)IOHIDElementGetReportID(e);
				break;
			}
		}
		CFRelease(els);
	}
	return report;
}

// zvm_led_value reads the host's current value of one LED usage (Num/Caps
// Lock), so a write does not clear what macOS believes is lit.
static int zvm_led_value(IOHIDDeviceRef dev, int usage) {
	CFMutableDictionaryRef m = CFDictionaryCreateMutable(kCFAllocatorDefault, 0,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	zvm_dict_set_int(m, CFSTR(kIOHIDElementUsagePageKey), 0x08);
	zvm_dict_set_int(m, CFSTR(kIOHIDElementUsageKey), usage);
	CFArrayRef els = IOHIDDeviceCopyMatchingElements(dev, m, kIOHIDOptionsTypeNone);
	CFRelease(m);
	int out = 0;
	if (els) {
		for (CFIndex i = 0; i < CFArrayGetCount(els); i++) {
			IOHIDElementRef e = (IOHIDElementRef)CFArrayGetValueAtIndex(els, i);
			if (IOHIDElementGetType(e) != kIOHIDElementTypeOutput) continue;
			IOHIDValueRef v = NULL;
			if (IOHIDDeviceGetValue(dev, e, &v) == kIOReturnSuccess && v) out = (int)IOHIDValueGetIntegerValue(v);
			break;
		}
		CFRelease(els);
	}
	return out;
}

// zvm_find_led returns the output element for one LED usage, retained, or NULL.
static IOHIDElementRef zvm_find_led(IOHIDDeviceRef dev, int usage) {
	CFMutableDictionaryRef m = CFDictionaryCreateMutable(kCFAllocatorDefault, 0,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	zvm_dict_set_int(m, CFSTR(kIOHIDElementUsagePageKey), 0x08);
	zvm_dict_set_int(m, CFSTR(kIOHIDElementUsageKey), usage);
	CFArrayRef els = IOHIDDeviceCopyMatchingElements(dev, m, kIOHIDOptionsTypeNone);
	CFRelease(m);
	IOHIDElementRef found = NULL;
	if (els) {
		for (CFIndex i = 0; i < CFArrayGetCount(els); i++) {
			IOHIDElementRef e = (IOHIDElementRef)CFArrayGetValueAtIndex(els, i);
			if (IOHIDElementGetType(e) == kIOHIDElementTypeOutput) { found = (IOHIDElementRef)CFRetain(e); break; }
		}
		CFRelease(els);
	}
	return found;
}

static void zvm_release_element(IOHIDElementRef e) { if (e) CFRelease(e); }
static int zvm_element_present(IOHIDElementRef e) { return e != NULL; }

static int zvm_element_value(IOHIDDeviceRef dev, IOHIDElementRef e) {
	if (!e) return 0;
	IOHIDValueRef v = NULL;
	if (IOHIDDeviceGetValue(dev, e, &v) != kIOReturnSuccess || !v) return 0;
	return (int)IOHIDValueGetIntegerValue(v);
}

// zvm_set_leds sets every LED element at once. SetValueMultiple is the path
// macOS keyboards honour (what screen readers and setledsmac use); it also
// emits a single output report, so the firmware never sees a half-written
// code. els/vals are parallel arrays of n entries; NULL elements are skipped.
static int zvm_set_leds(IOHIDDeviceRef dev, IOHIDElementRef *els, int *vals, int n) {
	CFMutableDictionaryRef d = CFDictionaryCreateMutable(kCFAllocatorDefault, 0,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	int any = 0;
	for (int i = 0; i < n; i++) {
		if (!els[i]) continue;
		IOHIDValueRef v = IOHIDValueCreateWithIntegerValue(kCFAllocatorDefault, els[i], 0, vals[i]);
		if (!v) continue;
		CFDictionarySetValue(d, els[i], v);
		CFRelease(v);
		any = 1;
	}
	IOReturn r = any ? IOHIDDeviceSetValueMultiple(dev, d) : kIOReturnNoResources;
	CFRelease(d);
	return (int)r;
}

static void zvm_retain(IOHIDDeviceRef dev) { CFRetain(dev); }
static void zvm_release(IOHIDDeviceRef dev) { CFRelease(dev); }
// zvm_scan_create returns every HID device the system knows, for diagnostics.
static CFArrayRef zvm_scan_create(void) {
	IOHIDManagerRef mgr = IOHIDManagerCreate(kCFAllocatorDefault, kIOHIDOptionsTypeNone);
	IOHIDManagerSetDeviceMatching(mgr, NULL);
	IOHIDManagerOpen(mgr, kIOHIDOptionsTypeNone);
	CFSetRef set = IOHIDManagerCopyDevices(mgr);
	CFArrayRef arr = NULL;
	if (set) {
		CFIndex n = CFSetGetCount(set);
		const void **vals = malloc(sizeof(void *) * (n > 0 ? n : 1));
		CFSetGetValues(set, vals);
		arr = CFArrayCreate(kCFAllocatorDefault, vals, n, &kCFTypeArrayCallBacks);
		free(vals);
		CFRelease(set);
	}
	IOHIDManagerClose(mgr, kIOHIDOptionsTypeNone);
	CFRelease(mgr);
	return arr;
}
static CFIndex zvm_array_count(CFArrayRef a) { return a ? CFArrayGetCount(a) : 0; }
static IOHIDDeviceRef zvm_array_at(CFArrayRef a, CFIndex i) { return (IOHIDDeviceRef)CFArrayGetValueAtIndex(a, i); }
static void zvm_array_free(CFArrayRef a) { if (a) CFRelease(a); }

static int zvm_open(IOHIDDeviceRef dev) { return (int)IOHIDDeviceOpen(dev, kIOHIDOptionsTypeNone); }
static void zvm_close(IOHIDDeviceRef dev) { IOHIDDeviceClose(dev, kIOHIDOptionsTypeNone); }
static int zvm_set_report(IOHIDDeviceRef dev, int report, uint8_t bits) {
	return (int)IOHIDDeviceSetReport(dev, kIOHIDReportTypeOutput, report, &bits, 1);
}
*/
import "C"

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"runtime/cgo"
	"strings"
	"sync"

	"github.com/rafaelromao/zmk-vim-mode/internal/leds"
)

// HID LED usages (page 0x08) and their bit in the 5-bit indicator report.
const (
	usageNumLock    = 0x01
	usageCapsLock   = 0x02
	usageScrollLock = 0x03
	usageCompose    = 0x04
	usageKana       = 0x05

	bitNumLock    = 0x01
	bitCapsLock   = 0x02
	bitScrollLock = 0x04
	bitCompose    = 0x08
	bitKana       = 0x10

	kIOReturnNotPermitted = -536870174 // 0xE00002E2
)

// Filter selects which HID devices to drive.
type Filter struct {
	VID, PID        uint16
	RequireCodeLEDs bool
	NameSubstring   string
}

// ledOrder is the element order used in device.els and in the value array
// handed to zvm_set_leds.
var ledOrder = [5]int{usageNumLock, usageCapsLock, usageScrollLock, usageCompose, usageKana}

type device struct {
	ref      C.IOHIDDeviceRef
	info     leds.Device
	reportID int
	opened   bool
	// els holds the output elements in ledOrder; a nil entry means the device
	// does not expose that LED.
	els [5]C.IOHIDElementRef
}

// Backend implements leds.Backend on IOKit.
type Backend struct {
	log    *slog.Logger
	f      Filter
	mu     sync.Mutex
	devs   map[leds.DeviceID]*device
	events chan<- leds.Event
	ctx    context.Context
	handle cgo.Handle
}

// New creates the backend.
func New(log *slog.Logger, f Filter) *Backend {
	if log == nil {
		log = slog.Default()
	}
	return &Backend{log: log, f: f, devs: map[leds.DeviceID]*device{}}
}

// Start runs the HID manager on its own OS thread until ctx is done.
func (b *Backend) Start(ctx context.Context, events chan<- leds.Event) error {
	b.ctx, b.events = ctx, events
	b.handle = cgo.NewHandle(b)
	go func() {
		runtime.LockOSThread()
		C.zvm_run(C.uintptr_t(b.handle), C.int(b.f.VID), C.int(b.f.PID))
	}()
	return nil
}

// Close stops the HID manager, which closes the devices. It is deliberately
// not tied to the context: the daemon writes a final OFF after its context is
// cancelled, and that write must still find the devices open.
func (b *Backend) Close() error {
	C.zvm_stop()
	return nil
}

// Devices lists the keyboards seen so far.
func (b *Backend) Devices() []leds.Device {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]leds.Device, 0, len(b.devs))
	for _, d := range b.devs {
		out = append(out, d.info)
	}
	return out
}

// Write sends the code, merged with the host's Num/Caps Lock state.
func (b *Backend) Write(id leds.DeviceID, code uint8) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	d, ok := b.devs[id]
	if !ok {
		return fmt.Errorf("device %s not present", id)
	}
	// A refused open is not fatal: the write is attempted anyway, since output
	// may go through where input monitoring does not. Retry the open on every
	// write, so granting the permission takes effect without a restart --
	// which matters because each rebuild of this binary voids the grant.
	if !d.opened {
		if r := int(C.zvm_open(d.ref)); r == 0 {
			d.opened = true
			d.info.Writable = true
			d.info.Note = ""
			b.collectElements(d)
			b.log.Info("device opened on retry (permission granted since start)", "dev", id, "product", d.info.Product)
		}
	}
	// Num and Caps Lock keep whatever the host has lit; the code owns the
	// other three.
	vals := [5]C.int{
		C.zvm_element_value(d.ref, d.els[0]),
		C.zvm_element_value(d.ref, d.els[1]),
		boolBit(code&4 != 0),
		boolBit(code&1 != 0),
		boolBit(code&2 != 0),
	}
	multi := int(C.zvm_set_leds(d.ref, &d.els[0], &vals[0], 5))
	if multi == 0 {
		return nil
	}
	// Fall back to a raw output report: some devices expose no writable
	// elements but accept the report.
	bits := codeToBits(code)
	if vals[0] != 0 {
		bits |= bitNumLock
	}
	if vals[1] != 0 {
		bits |= bitCapsLock
	}
	report := int(C.zvm_set_report(d.ref, C.int(d.reportID), C.uint8_t(bits)))
	if report == 0 {
		b.log.Debug("SetValueMultiple failed; SetReport worked", "dev", id, "err", ioReturn(multi))
		return nil
	}
	note := ""
	if !d.opened {
		note = " (device could not be opened: " + d.info.Note + ")"
	}
	return fmt.Errorf("SetValueMultiple: %s; SetReport: %s%s", ioReturn(multi), ioReturn(report), note)
}

// collectElements caches the LED output elements of a device.
func (b *Backend) collectElements(d *device) {
	for i, usage := range ledOrder {
		if C.zvm_element_present(d.els[i]) != 0 {
			continue
		}
		d.els[i] = C.zvm_find_led(d.ref, C.int(usage))
	}
}

func boolBit(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

// codeToBits maps the 3-bit code (b0 Compose, b1 Kana, b2 Scroll Lock) to
// report bits -- the same table as the Linux backend and the firmware.
func codeToBits(code uint8) uint8 {
	var bits uint8
	if code&1 != 0 {
		bits |= bitCompose
	}
	if code&2 != 0 {
		bits |= bitKana
	}
	if code&4 != 0 {
		bits |= bitScrollLock
	}
	return bits
}

// Common IOReturn codes, named so the log does not read as hex soup.
var ioReturnNames = map[uint32]string{
	0xE00002BC: "kIOReturnError (general)",
	0xE00002BD: "kIOReturnNoMemory",
	0xE00002BE: "kIOReturnNoResources",
	0xE00002C1: "kIOReturnNotPrivileged",
	0xE00002C2: "kIOReturnBadArgument",
	0xE00002C5: "kIOReturnExclusiveAccess (another process owns the device)",
	0xE00002C7: "kIOReturnUnsupported (the device does not accept this)",
	0xE00002CD: "kIOReturnNotOpen",
	0xE00002E2: "kIOReturnNotPermitted",
	0xE0005000: "HID system error (the system's keyboard driver refused it)",
}

func ioReturn(r int) string {
	u := uint32(int32(r))
	if u == 0xE00002E2 { // kIOReturnNotPermitted
		return "not permitted — grant Input Monitoring to ~/.local/bin/zmk-vim-mode " +
			"(System Settings → Privacy & Security; a rebuilt binary must be removed and re-added)"
	}
	if name, ok := ioReturnNames[u]; ok {
		return fmt.Sprintf("%s (0x%08x)", name, u)
	}
	return fmt.Sprintf("IOReturn 0x%08x", u)
}

func (b *Backend) emit(ev leds.Event) {
	select {
	case b.events <- ev:
	case <-b.ctx.Done():
	}
}

func prop(dev C.IOHIDDeviceRef, key *C.char) string {
	var buf [256]C.char
	cf := C.CFStringCreateWithCString(C.kCFAllocatorDefault, key, C.kCFStringEncodingUTF8)
	defer C.CFRelease(C.CFTypeRef(cf))
	C.zvm_prop_str(dev, cf, &buf[0], 256)
	return C.GoString(&buf[0])
}

func propInt(dev C.IOHIDDeviceRef, key *C.char) int {
	cf := C.CFStringCreateWithCString(C.kCFAllocatorDefault, key, C.kCFStringEncodingUTF8)
	defer C.CFRelease(C.CFTypeRef(cf))
	return int(C.zvm_prop_int(dev, cf))
}

var (
	keyProduct    = C.CString("Product")
	keyTransport  = C.CString("Transport")
	keyVendorID   = C.CString("VendorID")
	keyProductID  = C.CString("ProductID")
	keyUsagePage  = C.CString("PrimaryUsagePage")
	keyUsage      = C.CString("PrimaryUsage")
	keyManufactur = C.CString("Manufacturer")
)

// ScanEntry is one HID device as `hid-scan` reports it.
type ScanEntry struct {
	Product      string `json:"product"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Transport    string `json:"transport"`
	VID          uint16 `json:"vid"`
	PID          uint16 `json:"pid"`
	UsagePage    int    `json:"usage_page"`
	Usage        int    `json:"usage"`
	// Report id of each LED output element, -1 when the device has none.
	Compose    int `json:"compose_report"`
	Kana       int `json:"kana_report"`
	ScrollLock int `json:"scroll_report"`
	NumLock    int `json:"num_report"`
}

// CodeLEDs reports whether the device carries the three indicators the
// protocol needs.
func (e ScanEntry) CodeLEDs() bool { return e.Compose >= 0 && e.Kana >= 0 && e.ScrollLock >= 0 }

// Scan lists every HID device on the system. It needs no permission: the
// properties and element list are readable without opening the device.
func Scan() []ScanEntry {
	arr := C.zvm_scan_create()
	defer C.zvm_array_free(arr)
	n := int(C.zvm_array_count(arr))
	out := make([]ScanEntry, 0, n)
	for i := 0; i < n; i++ {
		dev := C.zvm_array_at(arr, C.CFIndex(i))
		out = append(out, ScanEntry{
			Product:      prop(dev, keyProduct),
			Manufacturer: prop(dev, keyManufactur),
			Transport:    prop(dev, keyTransport),
			VID:          uint16(propInt(dev, keyVendorID)),
			PID:          uint16(propInt(dev, keyProductID)),
			UsagePage:    propInt(dev, keyUsagePage),
			Usage:        propInt(dev, keyUsage),
			Compose:      int(C.zvm_led_element(dev, usageCompose)),
			Kana:         int(C.zvm_led_element(dev, usageKana)),
			ScrollLock:   int(C.zvm_led_element(dev, usageScrollLock)),
			NumLock:      int(C.zvm_led_element(dev, usageNumLock)),
		})
	}
	return out
}

//export zvmDeviceMatched
func zvmDeviceMatched(handle C.uintptr_t, dev C.IOHIDDeviceRef) {
	b := cgo.Handle(handle).Value().(*Backend)
	id := leds.DeviceID(fmt.Sprintf("hid-%x", uint64(C.zvm_entry_id(dev))))
	info := leds.Device{
		ID:      id,
		VID:     uint16(propInt(dev, keyVendorID)),
		PID:     uint16(propInt(dev, keyProductID)),
		Product: prop(dev, keyProduct),
	}
	switch strings.ToLower(prop(dev, keyTransport)) {
	case "usb":
		info.Transport = "usb"
	case "bluetooth", "bluetooth low energy":
		info.Transport = "bluetooth"
	default:
		info.Transport = "unknown"
	}
	if b.f.NameSubstring != "" && !strings.Contains(strings.ToLower(info.Product), strings.ToLower(b.f.NameSubstring)) {
		b.log.Debug("skipping keyboard", "product", info.Product, "reason", "name filter")
		return
	}
	report := int(C.zvm_led_element(dev, usageCompose))
	noCodeLEDs := report < 0 || int(C.zvm_led_element(dev, usageKana)) < 0
	if noCodeLEDs {
		// The element list is not always readable -- it fills in once the
		// process may monitor input -- so a vendor-filtered device is never
		// rejected for it: the VID/PID already identifies the keyboard, and a
		// write to the wrong report id fails loudly. Without a vendor filter
		// the LEDs are the only discriminator left, so there it still decides.
		if b.f.RequireCodeLEDs && b.f.VID == 0 && b.f.PID == 0 {
			b.log.Debug("skipping keyboard", "product", info.Product, "reason", "no Compose/Kana LED elements")
			return
		}
		report = 1
		info.Note = "Compose/Kana LED elements not visible (firmware without CONFIG_ZMK_HID_INDICATORS, or Input Monitoring not granted yet)"
		b.log.Info("keyboard has no visible Compose/Kana LED elements; driving it anyway",
			"product", info.Product, "transport", info.Transport,
			"vid", fmt.Sprintf("%04x", info.VID), "pid", fmt.Sprintf("%04x", info.PID))
	}
	d := &device{ref: dev, info: info, reportID: report}
	C.zvm_retain(dev)
	if r := int(C.zvm_open(dev)); r != 0 {
		d.info.Note = ioReturn(r)
	} else {
		d.opened = true
		d.info.Writable = true
	}
	// Elements are collected whether or not the open succeeded: the list may
	// be readable either way, and a write is attempted regardless.
	b.collectElements(d)
	if noCodeLEDs && C.zvm_element_present(d.els[3]) != 0 {
		if r := int(C.zvm_led_element(dev, usageCompose)); r >= 0 {
			d.reportID = r
			d.info.Note = ""
			b.log.Info("Compose LED element visible now", "product", info.Product, "report", r)
		}
	}
	b.mu.Lock()
	b.devs[id] = d
	b.mu.Unlock()
	b.log.Info("keyboard found", "id", id, "product", info.Product, "transport", info.Transport,
		"vid", fmt.Sprintf("%04x", info.VID), "pid", fmt.Sprintf("%04x", info.PID), "report", report, "writable", d.opened, "note", d.info.Note)
	b.emit(leds.Event{Kind: leds.Added, Dev: id})
}

//export zvmDeviceRemoved
func zvmDeviceRemoved(handle C.uintptr_t, dev C.IOHIDDeviceRef) {
	b := cgo.Handle(handle).Value().(*Backend)
	b.mu.Lock()
	var gone *device
	var id leds.DeviceID
	for k, d := range b.devs {
		if d.ref == dev {
			gone, id = d, k
			delete(b.devs, k)
			break
		}
	}
	b.mu.Unlock()
	if gone == nil {
		return
	}
	for _, e := range gone.els {
		C.zvm_release_element(e)
	}
	if gone.opened {
		C.zvm_close(gone.ref)
	}
	C.zvm_release(gone.ref)
	b.log.Info("keyboard removed", "id", id)
	b.emit(leds.Event{Kind: leds.Removed, Dev: id})
}

//export zvmSystemWake
func zvmSystemWake(handle C.uintptr_t) {
	b := cgo.Handle(handle).Value().(*Backend)
	b.log.Info("system woke; re-asserting")
	b.emit(leds.Event{Kind: leds.Wake})
}
