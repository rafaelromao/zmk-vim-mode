package install

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// The daemon runs as a per-user service: a launchd agent on macOS, a systemd
// user unit on Linux. Start, Stop and Restart drive whichever this platform
// has, so nobody needs launchctl's target syntax or its rules about which
// verb applies to an agent in which state.

const (
	// AgentLabel is the launchd label of the macOS agent.
	AgentLabel = "dev.rafaelromao.zmk-vim-mode"
	// UnitName is the systemd user unit on Linux.
	UnitName = "zmk-vim-mode.service"
)

// agentDomain is the launchd domain of this user's GUI session, and
// agentTarget the agent within it.
func agentDomain() string { return fmt.Sprintf("gui/%d", os.Getuid()) }
func agentTarget() string { return agentDomain() + "/" + AgentLabel }

// service is the user service as seen from one platform. goos, run and
// sleep are fields so tests can drive both platforms' logic from either.
type service struct {
	goos  string
	plist string // the agent's plist, which bootstrap loads (macOS)
	run   func(name string, args ...string) (string, error)
	sleep func(time.Duration)
}

func runCommand(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

func newService() (*service, error) {
	p, err := ServicePath()
	if err != nil {
		return nil, err
	}
	return &service{goos: runtime.GOOS, plist: p, run: runCommand, sleep: time.Sleep}, nil
}

// Start starts the service, leaving it alone if it is already running.
func Start(w io.Writer) error {
	s, err := newService()
	if err != nil {
		return err
	}
	return s.start(w)
}

// Stop stops the service. It stays stopped until Start, Restart or the next
// login, which starts it as usual.
func Stop(w io.Writer) error {
	s, err := newService()
	if err != nil {
		return err
	}
	return s.stop(w)
}

// Restart stops the service if it is running and starts it again. A new
// process is what picks up a new binary or a permission granted since the
// old one started.
func Restart(w io.Writer) error {
	s, err := newService()
	if err != nil {
		return err
	}
	return s.restart(w)
}

func (s *service) start(w io.Writer) error {
	switch s.goos {
	case "darwin":
		if _, running := s.agentState(); running {
			fmt.Fprintln(w, "already running")
			return nil
		}
		if _, err := s.reload(); err != nil {
			return err
		}
		fmt.Fprintln(w, "started")
		return nil
	case "linux":
		if s.unitActive() {
			fmt.Fprintln(w, "already running")
			return nil
		}
		if err := s.systemctl("start"); err != nil {
			return err
		}
		fmt.Fprintln(w, "started")
		return nil
	}
	return errUnsupported(s.goos)
}

func (s *service) stop(w io.Writer) error {
	switch s.goos {
	case "darwin":
		// Killing the process is not enough: the agent is KeepAlive, so
		// launchd would start it again at once. Unloading is what stops it.
		if loaded, _ := s.agentState(); !loaded {
			fmt.Fprintln(w, "not running")
			return nil
		}
		if err := s.launchctl("bootout", agentTarget()); err != nil {
			return err
		}
	case "linux":
		if !s.unitActive() {
			fmt.Fprintln(w, "not running")
			return nil
		}
		if err := s.systemctl("stop"); err != nil {
			return err
		}
	default:
		return errUnsupported(s.goos)
	}
	fmt.Fprintln(w, "stopped; it starts again at your next login, or with: zmk-vim-mode start")
	return nil
}

func (s *service) restart(w io.Writer) error {
	switch s.goos {
	case "darwin":
		wasLoaded, err := s.reload()
		if err != nil {
			return err
		}
		if !wasLoaded {
			fmt.Fprintln(w, "started (it was not running)")
			return nil
		}
	case "linux":
		if err := s.systemctl("restart"); err != nil {
			return err
		}
	default:
		return errUnsupported(s.goos)
	}
	fmt.Fprintln(w, "restarted")
	return nil
}

// agentState asks launchd whether the agent is loaded and whether its
// process is running. `launchctl print` fails for an agent that is not
// loaded, and reports "state = running" at the top level of one that runs.
func (s *service) agentState() (loaded, running bool) {
	out, err := s.run("launchctl", "print", agentTarget())
	if err != nil {
		return false, false
	}
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "\tstate = ") {
			return true, strings.TrimSpace(strings.TrimPrefix(l, "\tstate = ")) == "running"
		}
	}
	return true, false
}

// reload unloads the agent if launchd has it and loads it again from its
// plist, which RunAtLoad then starts. It is how the agent starts and
// restarts on macOS, in preference to kickstart: on 2026-10-02 launchd
// refused to kickstart a rebuilt binary ("spawn failed", exit 78: EX_CONFIG,
// the agent marked "needs LWCR update") and kept refusing until the agent
// was unloaded and loaded again, which worked at once. Loading also makes
// launchd read the plist again, which install has just rewritten.
func (s *service) reload() (wasLoaded bool, err error) {
	// Check before unloading anything: without the plist there is nothing
	// to load again, and a running daemon would be stopped for good.
	if _, err := os.Stat(s.plist); err != nil {
		return false, fmt.Errorf("the service is not installed (%s is missing): run `zmk-vim-mode install` or `make install`", TrimHome(s.plist))
	}
	wasLoaded, _ = s.agentState()
	if wasLoaded {
		if err := s.launchctl("bootout", agentTarget()); err != nil {
			return true, err
		}
		// bootout can return before launchd has forgotten the agent, and a
		// bootstrap then fails with "5: Input/output error".
		for i := 0; i < 30; i++ {
			if loaded, _ := s.agentState(); !loaded {
				break
			}
			s.sleep(100 * time.Millisecond)
		}
	}
	for attempt := 1; ; attempt++ {
		err = s.launchctl("bootstrap", agentDomain(), s.plist)
		if err == nil || attempt == 3 {
			return wasLoaded, err
		}
		s.sleep(300 * time.Millisecond)
	}
}

func (s *service) launchctl(args ...string) error {
	if out, err := s.run("launchctl", args...); err != nil {
		return fmt.Errorf("launchctl %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(out))
	}
	return nil
}

func (s *service) unitActive() bool {
	_, err := s.run("systemctl", "--user", "is-active", "--quiet", UnitName)
	return err == nil
}

func (s *service) systemctl(verb string) error {
	if out, err := s.run("systemctl", "--user", verb, UnitName); err != nil {
		return fmt.Errorf("systemctl --user %s %s: %v: %s", verb, UnitName, err, strings.TrimSpace(out))
	}
	return nil
}

func errUnsupported(goos string) error {
	return errors.New("start, stop and restart drive launchd (macOS) or systemd (Linux); on " + goos +
		" run `zmk-vim-mode daemon` yourself")
}
