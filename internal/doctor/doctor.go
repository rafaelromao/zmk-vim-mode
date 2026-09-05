// Package doctor reports whether the environment is set up correctly.
package doctor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/install"
	"github.com/rafaelromao/zmk-vim-mode/internal/proto"
	"github.com/rafaelromao/zmk-vim-mode/internal/server"
	"github.com/rafaelromao/zmk-vim-mode/internal/state"
)

type result int

const (
	ok result = iota
	warn
	bad
	info
)

func (r result) mark() string {
	switch r {
	case ok:
		return "  ok  "
	case warn:
		return " warn "
	case bad:
		return " fail "
	default:
		return " info "
	}
}

type check struct {
	name   string
	res    result
	detail string
	hint   string
}

// Run prints the diagnostics. It returns an error only if a check could not be
// performed at all; failing checks are reported, not returned.
func Run(w io.Writer, socket string) error {
	var checks []check
	add := func(name string, r result, detail, hint string) {
		checks = append(checks, check{name, r, detail, hint})
	}

	// 1. daemon reachable
	status, err := server.Request(socket, proto.Msg{V: proto.Version, T: proto.TStatus}, 2*time.Second)
	if err != nil {
		add("daemon", bad, "not reachable at "+install.TrimHome(socket), serviceHint())
	} else if status.Status != nil {
		st := status.Status
		add("daemon", ok, fmt.Sprintf("up %s, decision %s (code %d) — %s",
			(time.Duration(st.UptimeS)*time.Second), st.Mode, st.Code, st.Reason), "")
		if st.Frontmost != nil && !st.Frontmost.Known {
			add("focus backend", warn, "frontmost app unknown; decisions trust editor clients",
				"on Hyprland make sure the daemon runs inside the graphical session")
		} else if st.Frontmost != nil {
			add("focus backend", ok, "frontmost "+st.Frontmost.Class, "")
		}
		if len(st.Clients) == 0 {
			add("editor clients", warn, "none connected",
				"add the lazy.nvim spec (zmk-vim-mode install --nvim) and restart Neovim")
		} else {
			var parts []string
			for _, c := range st.Clients {
				parts = append(parts, fmt.Sprintf("#%d %s mode=%s", c.ID, c.Kind, c.Mode))
			}
			add("editor clients", ok, strings.Join(parts, ", "), "")
		}
		// 2. devices
		if len(st.Devices) == 0 {
			add("keyboards", bad, "no keyboard exposing Compose+Kana+Scroll LEDs",
				"is CONFIG_ZMK_HID_INDICATORS=y in the central/dongle .conf, and the keyboard connected?")
		} else {
			for _, d := range st.Devices {
				r, hint := ok, ""
				if !d.Writable {
					r, hint = bad, "install the udev rule: zmk-vim-mode install --udev"
				}
				last := "never written"
				if d.LastCode != nil {
					last = fmt.Sprintf("last code %d (%s)", *d.LastCode, state.ModeName(*d.LastCode))
				}
				detail := fmt.Sprintf("%s [%s %04x:%04x] %s", d.Product, d.Transport, d.VID, d.PID, last)
				if d.Note != "" {
					detail += " — " + d.Note
				}
				add("keyboard "+d.ID, r, detail, hint)
			}
		}
	}

	// 3. service installed
	if p, err := install.ServicePath(); err == nil {
		if _, err := os.Stat(p); err == nil {
			add("service unit", ok, install.TrimHome(p), "")
		} else {
			add("service unit", warn, "not installed", "run: zmk-vim-mode install")
		}
	}

	// 4. old watchers
	var old []string
	for _, p := range install.OldWatchers() {
		if _, err := os.Stat(p); err == nil {
			old = append(old, install.TrimHome(p))
		}
	}
	if len(old) > 0 {
		add("old watchers", warn, strings.Join(old, ", "),
			"retire them: they fight this daemon over Num Lock (Hammerspoon require, hyprland.conf exec-once, systemd unit)")
	} else {
		add("old watchers", ok, "none found", "")
	}

	// 5. tmux focus events
	if os.Getenv("TMUX") != "" || hasBinary("tmux") {
		out, err := exec.Command("tmux", "show", "-gv", "focus-events").Output()
		switch {
		case err != nil:
			add("tmux focus-events", info, "tmux not running", "")
		case strings.TrimSpace(string(out)) == "on":
			add("tmux focus-events", ok, "on", "")
		default:
			add("tmux focus-events", warn, "off",
				"add 'set -g focus-events on' to ~/.tmux.conf, else Neovim never sees FocusLost")
		}
	}

	// 6. Linux specifics
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/etc/udev/rules.d/60-zmk-vim-mode.rules"); err == nil {
			add("udev rule", ok, "/etc/udev/rules.d/60-zmk-vim-mode.rules", "")
		} else {
			add("udev rule", warn, "not installed",
				"zmk-vim-mode install --udev, then sudo udevadm control --reload-rules && sudo udevadm trigger")
		}
		if hasBinary("hyprctl") {
			out, err := exec.Command("hyprctl", "getoption", "input:numlock_by_default").Output()
			s := strings.ToLower(string(out))
			switch {
			case err != nil:
				add("hyprland numlock_by_default", info, "hyprctl not answering", "")
			case strings.Contains(s, "int: 1") || strings.Contains(s, "true"):
				add("hyprland numlock_by_default", warn, "enabled",
					"set it false: with a num-lock listener in the keymap it can re-trigger vim mode")
			default:
				add("hyprland numlock_by_default", ok, "disabled", "")
			}
		}
	}

	// Print
	fmt.Fprintf(w, "zmk-vim-mode doctor (%s)\n\n", runtime.GOOS)
	var fails, warns int
	for _, c := range checks {
		fmt.Fprintf(w, "[%s] %-26s %s\n", c.res.mark(), c.name, c.detail)
		if c.hint != "" && c.res != ok {
			fmt.Fprintf(w, "                                → %s\n", c.hint)
		}
		switch c.res {
		case bad:
			fails++
		case warn:
			warns++
		}
	}
	fmt.Fprintf(w, "\n%d failed, %d warnings, %d checks\n", fails, warns, len(checks))
	fmt.Fprintln(w, "\nnote: doctor cannot verify the firmware module is flashed (the LED channel is write-only).")
	fmt.Fprintln(w, "verify manually: zmk-vim-mode set insert   → the keyboard should switch to its INSERT layer.")
	return nil
}

// serviceHint tells the user how to find out *why* the daemon is not there,
// not just how to start it again: a socket that is missing usually means the
// daemon exited, and the reason is in the service log.
func serviceHint() string {
	if runtime.GOOS == "darwin" {
		return "run it in the foreground to see the error: zmk-vim-mode daemon --log-level debug" +
			" (service: launchctl bootstrap gui/$UID ~/Library/LaunchAgents/dev.rafaelromao.zmk-vim-mode.plist," +
			" log: ~/Library/Logs/zmk-vim-mode.log)"
	}
	return "run it in the foreground to see the error: zmk-vim-mode daemon --log-level debug" +
		" (service: systemctl --user status zmk-vim-mode; log: journalctl --user -u zmk-vim-mode -n 40)"
}

func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
