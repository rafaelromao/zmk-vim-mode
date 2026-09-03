package server

import (
	"bufio"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/proto"
)

type recHandler struct {
	mu     sync.Mutex
	msgs   []proto.Msg
	closed []ConnID
}

func (h *recHandler) OnMessage(id ConnID, m proto.Msg) (*proto.Msg, bool) {
	h.mu.Lock()
	h.msgs = append(h.msgs, m)
	h.mu.Unlock()
	switch m.T {
	case proto.THello:
		if m.V > proto.Version {
			return &proto.Msg{T: proto.TError, Err: proto.ErrUnsupportedVersion}, true
		}
		return &proto.Msg{V: proto.Version, T: proto.TWelcome, Daemon: "test", Code: proto.U8(1)}, false
	case proto.TStatus:
		return &proto.Msg{T: proto.TOK, Code: proto.U8(2)}, true
	}
	return nil, false
}

func (h *recHandler) OnClose(id ConnID) {
	h.mu.Lock()
	h.closed = append(h.closed, id)
	h.mu.Unlock()
}

func tmpSock(t *testing.T) string {
	// unix socket paths are limited (~104 bytes on macOS); keep it short.
	d, err := os.MkdirTemp("", "zvm")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	requireUnixBind(t, d)
	return filepath.Join(d, "s", "daemon.sock")
}

// requireUnixBind skips the test when the environment forbids binding unix
// sockets (some sandboxes/CI seccomp profiles do). The daemon itself surfaces
// the same error at runtime, so there is nothing to work around here.
func requireUnixBind(t *testing.T, dir string) {
	t.Helper()
	probe := filepath.Join(dir, "probe.sock")
	ln, err := net.Listen("unix", probe)
	if err != nil {
		if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
			t.Skipf("unix socket bind not permitted in this environment: %v", err)
		}
		t.Fatalf("unix socket probe failed: %v", err)
	}
	ln.Close()
	os.Remove(probe)
}

func TestListenPermissionsAndSingleInstance(t *testing.T) {
	path := tmpSock(t)
	h := &recHandler{}
	s, err := Listen(path, h, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if st, err := os.Stat(filepath.Dir(path)); err != nil || st.Mode().Perm() != 0o700 {
		t.Fatalf("dir perm: %v %v", st.Mode(), err)
	}
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("socket perm: %v %v", st.Mode(), err)
	}
	if _, err := Listen(path, h, nil); err != ErrAlreadyRunning {
		t.Fatalf("second listen: %v", err)
	}
}

func TestStaleSocketIsReplaced(t *testing.T) {
	path := tmpSock(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	// Leave a dead socket file behind.
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close()
	s, err := Listen(path, &recHandler{}, nil)
	if err != nil {
		t.Fatalf("stale socket should be replaced: %v", err)
	}
	s.Close()
}

func TestServeProtocol(t *testing.T) {
	path := tmpSock(t)
	h := &recHandler{}
	s, err := Listen(path, h, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()

	// hello → welcome, connection stays open
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := proto.Encode(proto.Msg{V: 1, T: proto.THello, Client: "nvim", Mode: "normal"})
	c.Write(b)
	r := bufio.NewReader(c)
	line, _ := r.ReadString('\n')
	if !strings.Contains(line, `"welcome"`) {
		t.Fatalf("expected welcome, got %q", line)
	}
	// malformed lines: tolerated up to 4, closed on the 5th
	for i := 0; i < 4; i++ {
		c.Write([]byte("garbage\n"))
	}
	b, _ = proto.Encode(proto.Msg{T: proto.TMode, Mode: "insert"})
	c.Write(b)
	time.Sleep(50 * time.Millisecond)
	h.mu.Lock()
	n := len(h.msgs)
	h.mu.Unlock()
	if n != 2 {
		t.Fatalf("expected 2 decoded messages, got %d", n)
	}
	c.Write([]byte("garbage\n"))
	c.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := r.ReadByte(); err == nil {
		t.Fatal("expected connection closed after 5 malformed lines")
	}
	c.Close()

	// Request helper: status → ok, then close by handler
	reply, err := Request(path, proto.Msg{V: 1, T: proto.TStatus}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if reply.T != proto.TOK || reply.Code == nil || *reply.Code != 2 {
		t.Fatalf("unexpected reply %+v", reply)
	}

	// version mismatch → error + close
	c2, _ := net.Dial("unix", path)
	b, _ = proto.Encode(proto.Msg{V: 99, T: proto.THello})
	c2.Write(b)
	line, _ = bufio.NewReader(c2).ReadString('\n')
	if !strings.Contains(line, proto.ErrUnsupportedVersion) {
		t.Fatalf("expected version error, got %q", line)
	}
	c2.Close()

	// oversized line → connection closed without reply
	c3, _ := net.Dial("unix", path)
	c3.Write([]byte(strings.Repeat("x", proto.MaxLine+10) + "\n"))
	c3.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := bufio.NewReader(c3).ReadByte(); err == nil {
		t.Fatal("expected close on oversized line")
	}
	c3.Close()

	time.Sleep(50 * time.Millisecond)
	h.mu.Lock()
	closed := len(h.closed)
	h.mu.Unlock()
	if closed < 4 {
		t.Fatalf("expected OnClose for each connection, got %d", closed)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after cancel")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("socket should be removed on close: %v", err)
	}
}
