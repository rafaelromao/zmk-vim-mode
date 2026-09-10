// Package hyprland watches the active window through Hyprland's IPC sockets.
//
// It never requires HYPRLAND_INSTANCE_SIGNATURE in the environment (the old
// watcher's "restart the computer" root cause): instances are discovered by
// globbing $XDG_RUNTIME_DIR/hypr/*/.socket2.sock, and the watcher reconnects
// when Hyprland restarts.
package hyprland

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
)

// Watcher implements focus.Watcher for Hyprland.
type Watcher struct {
	log        *slog.Logger
	RuntimeDir string        // default $XDG_RUNTIME_DIR or /run/user/<uid>
	Retry      time.Duration // reconnect/discovery interval
}

// New creates a Watcher.
func New(log *slog.Logger) *Watcher {
	if log == nil {
		log = slog.Default()
	}
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return &Watcher{log: log, RuntimeDir: dir, Retry: 2 * time.Second}
}

// Available reports whether a Hyprland instance socket can be found.
func (w *Watcher) Available() bool {
	_, err := w.discover()
	return err == nil
}

// Run streams frontmost-window changes until ctx is done. While no instance is
// reachable it emits an unknown App once (fail open) and keeps retrying.
func (w *Watcher) Run(ctx context.Context, emit func(focus.App)) error {
	unknownSent := false
	for ctx.Err() == nil {
		inst, err := w.discover()
		if err != nil {
			if !unknownSent {
				emit(focus.App{})
				unknownSent = true
			}
			w.log.Debug("hyprland instance not found", "err", err)
			if !sleep(ctx, w.Retry) {
				break
			}
			continue
		}
		unknownSent = false
		if err := w.stream(ctx, inst, emit); err != nil && ctx.Err() == nil {
			w.log.Info("hyprland event socket closed; reconnecting", "err", err)
			emit(focus.App{}) // unknown until we reconnect → decision fails open
			unknownSent = true
			sleep(ctx, w.Retry)
		}
	}
	return nil
}

// discover returns the instance directory of the most recent Hyprland instance.
func (w *Watcher) discover() (string, error) {
	if sig := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE"); sig != "" {
		d := filepath.Join(w.RuntimeDir, "hypr", sig)
		if _, err := os.Stat(filepath.Join(d, ".socket2.sock")); err == nil {
			return d, nil
		}
	}
	matches, _ := filepath.Glob(filepath.Join(w.RuntimeDir, "hypr", "*", ".socket2.sock"))
	if len(matches) == 0 {
		return "", fmt.Errorf("no .socket2.sock under %s/hypr", w.RuntimeDir)
	}
	type cand struct {
		dir string
		mod time.Time
	}
	var cands []cand
	for _, m := range matches {
		st, err := os.Stat(m)
		if err != nil {
			continue
		}
		dir := filepath.Dir(m)
		// Prefer instances that answer on the control socket.
		if _, err := ctl(dir, "j/version", time.Second); err != nil {
			continue
		}
		cands = append(cands, cand{dir, st.ModTime()})
	}
	if len(cands) == 0 {
		return "", fmt.Errorf("no responsive Hyprland instance under %s/hypr", w.RuntimeDir)
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].mod.After(cands[j].mod) })
	return cands[0].dir, nil
}

func (w *Watcher) stream(ctx context.Context, inst string, emit func(focus.App)) error {
	c, err := net.DialTimeout("unix", filepath.Join(inst, ".socket2.sock"), 2*time.Second)
	if err != nil {
		return err
	}
	defer c.Close()
	go func() {
		<-ctx.Done()
		c.Close()
	}()
	tr := &tracker{}
	// Initial state so we are not "unknown until the first switch".
	if app, addr, err := w.activeWindow(inst); err == nil {
		tr.note(app, addr)
		emit(app)
	} else {
		w.log.Debug("j/activewindow failed", "err", err)
	}
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if !tr.trigger(line) {
			continue
		}
		// The event is only the trigger; j/activewindow is the source of truth
		// (it carries the pid, which event lines lack, so the frontmost identity
		// stays stable). Fall back to the event's own class/title if the
		// control socket does not answer.
		app, addr, err := w.activeWindow(inst)
		if err != nil {
			w.log.Debug("j/activewindow failed", "err", err)
			var ok bool
			if app, ok = ParseEvent(line); !ok {
				continue
			}
		}
		if tr.note(app, addr) {
			emit(app)
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return io.EOF
}

// tracker decides which socket2 lines are worth a j/activewindow query and
// dedupes the results. Hyprland reports a focus change as
// `activewindow>>CLASS,TITLE` plus `activewindowv2>>ADDRESS`, and a title
// change as `windowtitle>>ADDRESS` (older) or `windowtitlev2>>ADDRESS,TITLE`.
// Title changes matter: VSCode publishes its focused view in the title. Only
// the focused window's titles are followed -- background shells and browsers
// retitle constantly.
type tracker struct {
	last  focus.App
	addr  string // focused window address, normalised (lower case, no 0x)
	sawV2 bool   // windowtitlev2 seen: the v1 line is a duplicate from then on
}

func (t *tracker) trigger(line string) bool {
	switch {
	case strings.HasPrefix(line, "activewindow>>"):
		return true
	case strings.HasPrefix(line, "activewindowv2>>"):
		t.addr = normAddr(strings.TrimPrefix(line, "activewindowv2>>"))
		return false
	case strings.HasPrefix(line, "windowtitlev2>>"):
		t.sawV2 = true
		addr, _, _ := strings.Cut(strings.TrimPrefix(line, "windowtitlev2>>"), ",")
		return t.isFocused(addr)
	case strings.HasPrefix(line, "windowtitle>>"):
		if t.sawV2 {
			return false
		}
		return t.isFocused(strings.TrimPrefix(line, "windowtitle>>"))
	}
	return false
}

// isFocused is true for the focused window's address, and also while the
// focused address is unknown (a query is cheap insurance then).
func (t *tracker) isFocused(addr string) bool {
	return t.addr == "" || normAddr(addr) == t.addr
}

// note records a query result and reports whether it differs from the last
// emitted one.
func (t *tracker) note(app focus.App, addr string) bool {
	if addr != "" {
		t.addr = normAddr(addr)
	}
	if app == t.last {
		return false
	}
	t.last = app
	return true
}

// normAddr makes event addresses (5581f0a1b2c0) and JSON ones (0x5581f0a1b2c0)
// comparable.
func normAddr(a string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(a)), "0x")
}

// ParseEvent parses one `activewindow>>CLASS,TITLE` line (CLASS cannot
// contain a comma; TITLE may — SplitN keeps it intact). An empty CLASS means
// no window is focused (desktop) → known, empty app. The stream uses it only
// as a fallback when the control socket does not answer.
func ParseEvent(line string) (focus.App, bool) {
	const prefix = "activewindow>>"
	if !strings.HasPrefix(line, prefix) {
		return focus.App{}, false
	}
	rest := line[len(prefix):]
	parts := strings.SplitN(rest, ",", 2)
	app := focus.App{Known: true, Class: parts[0]}
	if len(parts) == 2 {
		app.Title = parts[1]
	}
	return app, true
}

type activeWindowJSON struct {
	Address string `json:"address"`
	Class   string `json:"class"`
	Title   string `json:"title"`
	PID     int    `json:"pid"`
}

// activeWindow queries the focused window; the second result is its address.
func (w *Watcher) activeWindow(inst string) (focus.App, string, error) {
	out, err := ctl(inst, "j/activewindow", time.Second)
	if err != nil {
		return focus.App{}, "", err
	}
	out = strings.TrimSpace(out)
	if out == "" || out == "{}" || strings.HasPrefix(out, "Invalid") {
		return focus.App{Known: true}, "", nil
	}
	var aw activeWindowJSON
	if err := json.Unmarshal([]byte(out), &aw); err != nil {
		return focus.App{}, "", fmt.Errorf("parse activewindow: %w (%q)", err, truncate(out, 80))
	}
	return focus.App{Known: true, Class: aw.Class, Title: aw.Title, PID: aw.PID}, aw.Address, nil
}

// ctl sends one command to .socket.sock and returns the full reply.
func ctl(inst, cmd string, timeout time.Duration) (string, error) {
	c, err := net.DialTimeout("unix", filepath.Join(inst, ".socket.sock"), timeout)
	if err != nil {
		return "", err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(timeout))
	if _, err := c.Write([]byte(cmd)); err != nil {
		return "", err
	}
	b, err := io.ReadAll(c)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
