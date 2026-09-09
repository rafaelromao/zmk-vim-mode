// Package server implements the daemon's unix-socket endpoint: one JSON
// message per line, one goroutine per connection, 0600 socket in a 0700
// directory, single-instance lock.
package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/proto"
)

// ErrAlreadyRunning is returned by Listen when another daemon owns the socket.
var ErrAlreadyRunning = errors.New("another zmk-vim-mode daemon is already running")

// ConnID identifies a connection for the lifetime of the server.
type ConnID = uint64

// Handler receives decoded messages. OnMessage may return a reply to write back
// and whether to close the connection afterwards. OnClose is called exactly once
// per connection that reached the handler.
type Handler interface {
	OnMessage(id ConnID, m proto.Msg) (reply *proto.Msg, closeAfter bool)
	OnClose(id ConnID)
}

// DefaultSocketPath returns $HOME/.local/state/zmk-vim-mode/daemon.sock. XDG is
// deliberately ignored so launchd/systemd/tmux-hosted clients all agree.
func DefaultSocketPath() string {
	if p := os.Getenv("ZMK_VIM_MODE_SOCKET"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.TempDir()
	}
	return filepath.Join(home, ".local", "state", "zmk-vim-mode", "daemon.sock")
}

// Server is a listening unix socket.
type Server struct {
	path    string
	ln      net.Listener
	lock    *os.File
	h       Handler
	log     *slog.Logger
	nextID  atomic.Uint64
	mu      sync.Mutex
	conns   map[ConnID]*conn
	wg      sync.WaitGroup
	closing atomic.Bool
}

type conn struct {
	id ConnID
	c  net.Conn
	mu sync.Mutex // serialises writes
}

const (
	maxMalformed = 5
	writeTimeout = time.Second
	// retryInterval paces Request's redial attempts while a daemon starts up.
	retryInterval = 100 * time.Millisecond
)

// startingUp reports whether a dial error is the kind a daemon that is still
// coming up produces: the socket file is not there yet, or nothing is
// listening on it. Anything else (permission denied, for instance) is final.
func startingUp(err error) bool {
	return errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ECONNREFUSED)
}

// Listen prepares the socket: creates the directory (0700), takes the
// single-instance lock, removes a stale socket (only if nobody answers on it)
// and listens with mode 0600.
func Listen(path string, h Handler, log *slog.Logger) (*Server, error) {
	if log == nil {
		log = slog.Default()
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	_ = os.Chmod(dir, 0o700)

	lock, err := os.OpenFile(filepath.Join(dir, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, ErrAlreadyRunning
	}

	if _, err := os.Stat(path); err == nil {
		// Something is there. If it answers, another daemon is alive despite the lock.
		if c, err := net.DialTimeout("unix", path, 300*time.Millisecond); err == nil {
			c.Close()
			lock.Close()
			return nil, ErrAlreadyRunning
		}
		if err := os.Remove(path); err != nil {
			lock.Close()
			return nil, fmt.Errorf("remove stale socket: %w", err)
		}
	}

	old := syscall.Umask(0o077)
	ln, err := net.Listen("unix", path)
	syscall.Umask(old)
	if err != nil {
		lock.Close()
		return nil, fmt.Errorf("listen %s: %w", path, err)
	}
	_ = os.Chmod(path, 0o600)

	return &Server{path: path, ln: ln, lock: lock, h: h, log: log, conns: map[ConnID]*conn{}}, nil
}

// Path returns the socket path.
func (s *Server) Path() string { return s.path }

// Serve accepts connections until ctx is done or Close is called.
func (s *Server) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		s.Close()
	}()
	for {
		c, err := s.ln.Accept()
		if err != nil {
			if s.closing.Load() || ctx.Err() != nil {
				s.wg.Wait()
				return nil
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			return err
		}
		id := s.nextID.Add(1)
		cc := &conn{id: id, c: c}
		s.mu.Lock()
		s.conns[id] = cc
		s.mu.Unlock()
		s.wg.Add(1)
		go s.serveConn(cc)
	}
}

func (s *Server) serveConn(cc *conn) {
	defer s.wg.Done()
	defer func() {
		cc.c.Close()
		s.mu.Lock()
		delete(s.conns, cc.id)
		s.mu.Unlock()
		s.h.OnClose(cc.id)
	}()
	sc := bufio.NewScanner(cc.c)
	sc.Buffer(make([]byte, 0, 4096), proto.MaxLine+2)
	malformed := 0
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		m, err := proto.Decode(line)
		if err != nil {
			malformed++
			s.log.Debug("malformed message", "conn", cc.id, "err", err)
			if malformed >= maxMalformed {
				s.log.Warn("closing connection after repeated malformed messages", "conn", cc.id)
				return
			}
			continue
		}
		reply, closeAfter := s.h.OnMessage(cc.id, m)
		if reply != nil {
			if err := s.write(cc, *reply); err != nil {
				s.log.Debug("write failed", "conn", cc.id, "err", err)
				return
			}
		}
		if closeAfter {
			return
		}
	}
	if err := sc.Err(); err != nil && !s.closing.Load() {
		s.log.Debug("connection read ended", "conn", cc.id, "err", err)
	}
}

func (s *Server) write(cc *conn, m proto.Msg) error {
	b, err := proto.Encode(m)
	if err != nil {
		return err
	}
	cc.mu.Lock()
	defer cc.mu.Unlock()
	_ = cc.c.SetWriteDeadline(time.Now().Add(writeTimeout))
	_, err = cc.c.Write(b)
	return err
}

// Send writes a message to one connection (e.g. resync).
func (s *Server) Send(id ConnID, m proto.Msg) error {
	s.mu.Lock()
	cc, ok := s.conns[id]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("connection %d not found", id)
	}
	return s.write(cc, m)
}

// Broadcast writes a message to every connection; failures are logged and ignored.
func (s *Server) Broadcast(m proto.Msg) {
	s.mu.Lock()
	all := make([]*conn, 0, len(s.conns))
	for _, cc := range s.conns {
		all = append(all, cc)
	}
	s.mu.Unlock()
	for _, cc := range all {
		if err := s.write(cc, m); err != nil {
			s.log.Debug("broadcast write failed", "conn", cc.id, "err", err)
		}
	}
}

// Close stops accepting, closes all connections, removes the socket and
// releases the lock. Safe to call more than once.
func (s *Server) Close() error {
	if !s.closing.CompareAndSwap(false, true) {
		return nil
	}
	err := s.ln.Close()
	s.mu.Lock()
	for _, cc := range s.conns {
		cc.c.Close()
	}
	s.mu.Unlock()
	_ = os.Remove(s.path)
	if s.lock != nil {
		_ = syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
		s.lock.Close()
	}
	return err
}

// Request dials the daemon, sends one message and returns the first reply.
// Used by the CLI.
//
// Dialling is retried within the timeout budget: with Type=simple, systemctl
// returns as soon as the daemon is spawned, so a command run right after
// `systemctl restart` would otherwise report a perfectly healthy daemon as
// unreachable.
func Request(path string, m proto.Msg, timeout time.Duration) (proto.Msg, error) {
	deadline := time.Now().Add(timeout)
	var c net.Conn
	var err error
	for {
		c, err = net.DialTimeout("unix", path, time.Until(deadline))
		if err == nil {
			break
		}
		if !startingUp(err) || !time.Now().Add(retryInterval).Before(deadline) {
			return proto.Msg{}, fmt.Errorf("daemon not reachable at %s: %w", path, err)
		}
		time.Sleep(retryInterval)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(timeout))
	b, err := proto.Encode(m)
	if err != nil {
		return proto.Msg{}, err
	}
	if _, err := c.Write(b); err != nil {
		return proto.Msg{}, err
	}
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 0, 4096), proto.MaxLine+2)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return proto.Msg{}, err
		}
		return proto.Msg{}, errors.New("daemon closed the connection without a reply")
	}
	return proto.Decode(sc.Bytes())
}
