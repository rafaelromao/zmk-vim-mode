package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var errExit = errors.New("exit status 1")

// fakeLaunchd stands in for launchctl, keeping the one agent's state: print
// fails for an agent that is not loaded, bootout unloads it, bootstrap loads
// and (RunAtLoad) starts it.
type fakeLaunchd struct {
	loaded, running bool
	// failBootstraps makes that many bootstraps fail the way one does while
	// launchd is still tearing down the previous instance.
	failBootstraps int
	calls          []string
}

func (f *fakeLaunchd) run(name string, args ...string) (string, error) {
	f.calls = append(f.calls, strings.Join(append([]string{name}, args...), " "))
	switch args[0] {
	case "print":
		if !f.loaded {
			return "Could not find service in domain\n", errExit
		}
		state := "not running"
		if f.running {
			state = "running"
		}
		// The shape macOS prints: the agent's own state is the one at the top
		// level, one tab in; nested blocks have states of their own.
		return "gui/501/" + AgentLabel + " = {\n\tpath = /x.plist\n\tstate = " + state +
			"\n\tendpoints = {\n\t\tstate = active\n\t}\n}\n", nil
	case "bootout":
		f.loaded, f.running = false, false
	case "bootstrap":
		if f.failBootstraps > 0 {
			f.failBootstraps--
			return "Bootstrap failed: 5: Input/output error\n", errExit
		}
		f.loaded, f.running = true, true
	case "kickstart":
		f.running = true
	}
	return "", nil
}

func (f *fakeLaunchd) ran(verb string) int {
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, "launchctl "+verb+" ") {
			n++
		}
	}
	return n
}

func darwinService(t *testing.T, f *fakeLaunchd) *service {
	t.Helper()
	plist := filepath.Join(t.TempDir(), AgentLabel+".plist")
	if err := os.WriteFile(plist, []byte("<plist/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	return &service{goos: "darwin", plist: plist, run: f.run, sleep: func(time.Duration) {}}
}

// After a rebuild launchd refused to kickstart the new binary until the agent
// was unloaded and loaded again, so starting anything that is not running is
// a reload, never a kickstart.
func TestDarwinStartReloadsAnythingNotRunning(t *testing.T) {
	cases := []struct {
		name                 string
		loaded, running      bool
		bootouts, bootstraps int
		says                 string
	}{
		{"not loaded", false, false, 0, 1, "started"},
		{"loaded, not running", true, false, 1, 1, "started"},
		{"running", true, true, 0, 0, "already running"},
	}
	for _, c := range cases {
		f := &fakeLaunchd{loaded: c.loaded, running: c.running}
		s := darwinService(t, f)
		var out strings.Builder
		if err := s.start(&out); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if f.ran("bootout") != c.bootouts || f.ran("bootstrap") != c.bootstraps || f.ran("kickstart") != 0 {
			t.Errorf("%s: calls %q", c.name, f.calls)
		}
		if !strings.Contains(out.String(), c.says) || !f.running {
			t.Errorf("%s: said %q, running=%v", c.name, out.String(), f.running)
		}
	}
}

func TestDarwinRestartReloadsTheAgent(t *testing.T) {
	f := &fakeLaunchd{loaded: true, running: true}
	s := darwinService(t, f)
	var out strings.Builder
	if err := s.restart(&out); err != nil {
		t.Fatal(err)
	}
	if f.ran("bootout") != 1 || f.ran("bootstrap") != 1 || f.ran("kickstart") != 0 {
		t.Errorf("calls %q", f.calls)
	}
	if want := "launchctl bootstrap " + agentDomain() + " " + s.plist; f.calls[len(f.calls)-1] != want {
		t.Errorf("last call %q, want %q", f.calls[len(f.calls)-1], want)
	}
	if !strings.Contains(out.String(), "restarted") {
		t.Errorf("said %q", out.String())
	}

	f = &fakeLaunchd{}
	s = darwinService(t, f)
	out.Reset()
	if err := s.restart(&out); err != nil {
		t.Fatal(err)
	}
	if f.ran("bootout") != 0 || f.ran("bootstrap") != 1 || !strings.Contains(out.String(), "not running") {
		t.Errorf("not loaded: calls %q, said %q", f.calls, out.String())
	}
}

// Without the plist there is nothing to load again: the running agent must
// not be unloaded first.
func TestDarwinReloadWithoutAPlistLeavesTheAgentAlone(t *testing.T) {
	f := &fakeLaunchd{loaded: true, running: true}
	s := &service{goos: "darwin", plist: filepath.Join(t.TempDir(), "missing.plist"), run: f.run, sleep: func(time.Duration) {}}
	err := s.restart(&strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "install") {
		t.Errorf("err = %v", err)
	}
	if f.ran("bootout") != 0 || !f.running {
		t.Errorf("the agent was stopped: %q", f.calls)
	}
}

func TestDarwinReloadRetriesABootstrapThatRacesTheBootout(t *testing.T) {
	f := &fakeLaunchd{loaded: true, running: true, failBootstraps: 2}
	if err := darwinService(t, f).restart(&strings.Builder{}); err != nil {
		t.Fatalf("two failed bootstraps should be retried: %v", err)
	}
	if f.ran("bootstrap") != 3 || !f.running {
		t.Errorf("calls %q", f.calls)
	}

	f = &fakeLaunchd{loaded: true, running: true, failBootstraps: 99}
	err := darwinService(t, f).restart(&strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "Input/output error") {
		t.Fatalf("err = %v", err)
	}
	if f.ran("bootstrap") != 3 {
		t.Errorf("gave up after %d bootstraps, want 3", f.ran("bootstrap"))
	}
}

// A KeepAlive agent comes straight back when its process dies, so stop must
// unload it rather than kill it.
func TestDarwinStopUnloadsTheAgent(t *testing.T) {
	f := &fakeLaunchd{loaded: true, running: true}
	if err := darwinService(t, f).stop(&strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	if f.ran("bootout") != 1 || f.loaded {
		t.Errorf("calls %q", f.calls)
	}

	f = &fakeLaunchd{}
	var out strings.Builder
	if err := darwinService(t, f).stop(&out); err != nil || f.ran("bootout") != 0 || !strings.Contains(out.String(), "not running") {
		t.Errorf("not loaded: err %v, calls %q, said %q", err, f.calls, out.String())
	}
}

// fakeSystemd records systemctl calls; is-active answers from active.
type fakeSystemd struct {
	active bool
	calls  []string
}

func (f *fakeSystemd) run(name string, args ...string) (string, error) {
	line := strings.Join(append([]string{name}, args...), " ")
	f.calls = append(f.calls, line)
	if strings.Contains(line, " is-active ") && !f.active {
		return "", errExit
	}
	return "", nil
}

func TestLinuxServiceCommands(t *testing.T) {
	active := "systemctl --user is-active --quiet " + UnitName
	cases := []struct {
		name   string
		active bool
		do     func(*service) error
		want   string // the last command run
	}{
		{"start", false, func(s *service) error { return s.start(&strings.Builder{}) }, "systemctl --user start " + UnitName},
		{"start running", true, func(s *service) error { return s.start(&strings.Builder{}) }, active},
		{"stop", true, func(s *service) error { return s.stop(&strings.Builder{}) }, "systemctl --user stop " + UnitName},
		{"stop stopped", false, func(s *service) error { return s.stop(&strings.Builder{}) }, active},
		{"restart", false, func(s *service) error { return s.restart(&strings.Builder{}) }, "systemctl --user restart " + UnitName},
	}
	for _, c := range cases {
		f := &fakeSystemd{active: c.active}
		s := &service{goos: "linux", run: f.run, sleep: func(time.Duration) {}}
		if err := c.do(s); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
		if got := f.calls[len(f.calls)-1]; got != c.want {
			t.Errorf("%s: ran %q, want %q", c.name, f.calls, c.want)
		}
	}
}

func TestServiceCommandsNeedLaunchdOrSystemd(t *testing.T) {
	s := &service{goos: "freebsd", run: (&fakeSystemd{}).run, sleep: func(time.Duration) {}}
	if err := s.start(&strings.Builder{}); err == nil || !strings.Contains(err.Error(), "zmk-vim-mode daemon") {
		t.Fatalf("err = %v", err)
	}
}
