package leds

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/state"
)

type write struct {
	dev  DeviceID
	code uint8
}

type fakeBackend struct {
	mu      sync.Mutex
	devs    []Device
	writes  []write
	failFor map[DeviceID]int // remaining failures per device
	events  chan<- Event
}

func newFake(ids ...DeviceID) *fakeBackend {
	f := &fakeBackend{failFor: map[DeviceID]int{}}
	for _, id := range ids {
		f.devs = append(f.devs, Device{ID: id, Product: string(id), Writable: true})
	}
	return f
}

func (f *fakeBackend) Start(ctx context.Context, events chan<- Event) error {
	f.events = events
	for _, d := range f.devs {
		events <- Event{Kind: Added, Dev: d.ID}
	}
	return nil
}

func (f *fakeBackend) Devices() []Device {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Device(nil), f.devs...)
}

func (f *fakeBackend) Write(dev DeviceID, code uint8) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if n := f.failFor[dev]; n > 0 {
		f.failFor[dev] = n - 1
		return errors.New("EACCES")
	}
	f.writes = append(f.writes, write{dev, code})
	return nil
}

func (f *fakeBackend) got() []write {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]write(nil), f.writes...)
}

func settle() { time.Sleep(60 * time.Millisecond) }

func TestCoalesceAndDedupe(t *testing.T) {
	f := newFake("kb")
	r := NewReconciler(f, nil)
	r.SetDesired(1)
	r.SetDesired(5)
	r.SetDesired(2)
	settle()
	if w := f.got(); len(w) != 1 || w[0].code != 2 {
		t.Fatalf("3 SetDesired in 10 ms must yield one write with the last value, got %v", w)
	}
	r.SetDesired(2)
	settle()
	if w := f.got(); len(w) != 1 {
		t.Fatalf("unchanged code must not be written again: %v", w)
	}
	r.SetDesired(0)
	settle()
	if w := f.got(); len(w) != 2 || w[1].code != 0 {
		t.Fatalf("transition to 0: %v", w)
	}
}

func TestReassertUsesSilentAlias(t *testing.T) {
	f := newFake("kb")
	r := NewReconciler(f, nil)
	r.Coalesce = 0
	r.SetDesired(state.CodeLegacy)
	if w := f.got(); len(w) != 1 || w[0].code != 4 {
		t.Fatalf("transition to legacy writes 4: %v", w)
	}
	r.Reassert("kb", true, "test")
	if w := f.got(); len(w) != 2 || w[1].code != 7 {
		t.Fatalf("re-assert of legacy writes 7: %v", w)
	}
	// desired still legacy; a non-forced transition write is deduped against the alias
	r.SetDesired(state.CodeLegacy)
	if w := f.got(); len(w) != 2 {
		t.Fatalf("device showing 7 is equivalent to 4; no write expected: %v", w)
	}
	r.SetDesired(state.CodeInsert)
	r.Reassert("kb", true, "test")
	if w := f.got(); len(w) != 4 || w[2].code != 2 || w[3].code != 2 {
		t.Fatalf("non-legacy alias is identity: %v", w)
	}
}

func TestEventsAddedEchoRemoved(t *testing.T) {
	f := newFake("kb")
	r := NewReconciler(f, nil)
	r.Coalesce = 0
	r.EchoMinGap = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan Event, 16)
	go r.Run(ctx, events)

	// Added before any desired → nothing
	events <- Event{Kind: Added, Dev: "kb"}
	settle()
	if len(f.got()) != 0 {
		t.Fatalf("no desired yet, no write: %v", f.got())
	}
	r.SetDesired(1)
	if len(f.got()) != 1 {
		t.Fatalf("transition write: %v", f.got())
	}
	// echo equal-or-not: always a forced re-write, rate limited
	for i := 0; i < 10; i++ {
		events <- Event{Kind: Echo, Dev: "kb"}
	}
	time.Sleep(100 * time.Millisecond)
	n := len(f.got())
	if n < 2 || n > 3 {
		t.Fatalf("10 echoes within the gap must collapse to 1-2 writes, got %d: %v", n-1, f.got())
	}
	// device re-added → forced re-assert
	events <- Event{Kind: Added, Dev: "kb"}
	settle()
	if len(f.got()) != n+1 {
		t.Fatalf("added → one write, got %v", f.got())
	}
	// removed → forgotten; wake → reassert
	events <- Event{Kind: Removed, Dev: "kb"}
	events <- Event{Kind: Wake}
	settle()
	if len(f.got()) != n+2 {
		t.Fatalf("wake → one write, got %v", f.got())
	}
}

func TestRetriesOnError(t *testing.T) {
	f := newFake("kb")
	f.failFor["kb"] = 2
	r := NewReconciler(f, nil)
	r.Coalesce = 0
	r.RetryDelays = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	r.SetDesired(3)
	if len(f.got()) != 0 {
		t.Fatal("first two writes should fail")
	}
	time.Sleep(80 * time.Millisecond)
	if w := f.got(); len(w) != 1 || w[0].code != 3 {
		t.Fatalf("expected one successful retry, got %v", w)
	}
	// give up after all retries
	f.failFor["kb"] = 10
	r.SetDesired(2)
	time.Sleep(120 * time.Millisecond)
	if w := f.got(); len(w) != 1 {
		t.Fatalf("should give up silently, got %v", w)
	}
}

func TestWriteAllAndDevices(t *testing.T) {
	f := newFake("a", "b")
	r := NewReconciler(f, nil)
	r.Coalesce = 0
	r.SetDesired(6)
	r.WriteAll(0)
	w := f.got()
	if len(w) != 4 {
		t.Fatalf("2 transition + 2 write-all: %v", w)
	}
	devs := r.Devices()
	if len(devs) != 2 || devs[0].ID != "a" || devs[0].LastCode == nil || *devs[0].LastCode != 0 {
		t.Fatalf("devices: %+v", devs)
	}
}
