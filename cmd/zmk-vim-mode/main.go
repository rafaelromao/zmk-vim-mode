// Command zmk-vim-mode keeps a ZMK keyboard's layers in sync with the editor's
// vim state. See PLAN.md at the repository root.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/daemon"
	"github.com/rafaelromao/zmk-vim-mode/internal/doctor"
	"github.com/rafaelromao/zmk-vim-mode/internal/install"
	"github.com/rafaelromao/zmk-vim-mode/internal/proto"
	"github.com/rafaelromao/zmk-vim-mode/internal/server"
	"github.com/rafaelromao/zmk-vim-mode/internal/state"
)

// Version is set by the Makefile via -ldflags.
var Version = "dev"

const usageText = `zmk-vim-mode — sync ZMK keyboard layers with the editor's vim state

Usage:
  zmk-vim-mode daemon [flags]           run the daemon (normally via systemd/launchd)
  zmk-vim-mode set <mode> [--ttl 30s] [--sticky]
                                        manual override: off|normal|insert|visual|cmdline|raw|legacy|auto
                                        (re-issuing the same mode toggles back to auto)
  zmk-vim-mode status [--json]          show decision, frontmost app, clients, devices
  zmk-vim-mode devices                  list keyboards the daemon can write to
  zmk-vim-mode install [--nvim] [--tmux] [--udev] [--vscode] [--atspi]
                                        install the user service; print editor/tmux snippets;
                                        --vscode writes settings.json and installs the extensions;
                                        --atspi makes the service follow focus inside VSCode
  zmk-vim-mode uninstall
  zmk-vim-mode atspi-watch              print accessibility-bus focus events with the classifier's verdict
  zmk-vim-mode doctor                   check permissions, devices, old watchers, tmux, udev
  zmk-vim-mode version

Environment: ZMK_VIM_MODE_SOCKET overrides the socket path (default ~/.local/state/zmk-vim-mode/daemon.sock).
`

// ZMK's default USB/BLE identifiers, shared by every stock ZMK board and by
// the udev rule this project ships.
const (
	zmkVendorID  = 0x1d50
	zmkProductID = 0x615e
)

type deviceFilter struct {
	vid, pid    uint16
	name        string
	anyKeyboard bool
}

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "daemon":
		err = runDaemon(args)
	case "set":
		err = runSet(args)
	case "status":
		err = runStatus(args)
	case "devices":
		err = runDevices(args)
	case "install":
		err = runInstall(args)
	case "uninstall":
		err = install.Uninstall(os.Stdout)
	case "doctor":
		err = doctor.Run(os.Stdout, server.DefaultSocketPath(), Version)
	case "atspi-watch":
		err = runATSPIWatch(args)
	case "version", "--version", "-v":
		fmt.Printf("zmk-vim-mode %s (%s)\n", Version, platformName)
	case "help", "-h", "--help":
		fmt.Print(usageText)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usageText)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newLogger(level string) (*slog.Logger, error) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("bad --log-level %q", level)
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})), nil
}

func runDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	sock := fs.String("socket", server.DefaultSocketPath(), "unix socket path")
	level := fs.String("log-level", "info", "debug|info|warn|error")
	var legacy, terminals, guis stringList
	fs.Var(&legacy, "legacy-app", "extra legacy app class, optionally kind:class (repeatable)")
	fs.Var(&terminals, "terminal-app", "extra terminal app class (repeatable)")
	fs.Var(&guis, "gui-nvim-app", "extra Neovim GUI app class (repeatable)")
	noTitle := fs.Bool("no-title-heuristic", false, "disable 'nvim in window title → legacy' fallback")
	vid := fs.String("vid", "", "vendor id to drive, hex (default ZMK's 1d50)")
	pid := fs.String("pid", "", "product id to drive, hex (default ZMK's 615e)")
	name := fs.String("device-name", "", "only drive keyboards whose name contains this")
	anyKb := fs.Bool("any-keyboard", false, "do not require Compose+Kana+Scroll LED capability")
	anyVendor := fs.Bool("any-vendor", false,
		"drive any vendor's keyboards, not just ZMK's 1d50:615e — most keyboards declare the same "+
			"five LED indicators, so this will write to unrelated keyboards too")
	offDelay := fs.Duration("startup-off-delay", 1500*time.Millisecond, "delay before the first OFF write after start")
	atspiOn := fs.Bool("atspi", false,
		"follow keyboard focus inside VSCode through the accessibility bus (AT-SPI2); "+
			"turns accessibility on for every application of the session, as a screen reader would")
	if err := fs.Parse(args); err != nil {
		return err
	}
	log, err := newLogger(*level)
	if err != nil {
		return err
	}
	rules := state.DefaultRules()
	for _, l := range legacy {
		kind, class := "", l
		if i := strings.IndexByte(l, ':'); i > 0 {
			kind, class = l[:i], l[i+1:]
		}
		rules.LegacyApps = append(rules.LegacyApps, state.AppRule{Kind: kind, Classes: []string{class}})
	}
	rules.TerminalApps = append(rules.TerminalApps, terminals...)
	rules.GUINvimApps = append(rules.GUINvimApps, guis...)
	if *noTitle {
		rules.TitleLegacy = nil
	}
	// Default to ZMK's identifiers. LED capability alone is not a usable
	// discriminator: ordinary keyboards declare the same five indicators, so a
	// capability-only filter writes to unrelated devices.
	f := deviceFilter{name: *name, anyKeyboard: *anyKb, vid: zmkVendorID, pid: zmkProductID}
	if *anyVendor {
		f.vid, f.pid = 0, 0
	}
	if *vid != "" {
		if f.vid, err = parseHex16(*vid); err != nil {
			return fmt.Errorf("--vid: %w", err)
		}
	}
	if *pid != "" {
		if f.pid, err = parseHex16(*pid); err != nil {
			return fmt.Errorf("--pid: %w", err)
		}
	}

	d, err := daemon.New(daemon.Options{
		SocketPath: *sock, Rules: rules, Log: log, Version: Version, StartupOffDelay: *offDelay,
		Backend: newBackend(log, f), Focus: newFocusWatcher(log), Widget: newWidgetWatcher(log, *atspiOn),
	})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	err = d.Run(ctx)
	if errors.Is(err, server.ErrAlreadyRunning) {
		return err
	}
	return err
}

func parseHex16(s string) (uint16, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(s), "0x"), 16, 16)
	return uint16(v), err
}

func runSet(args []string) error {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	ttl := fs.Duration("ttl", 0, "expire the override after this duration")
	sticky := fs.Bool("sticky", false, "keep the override when the frontmost app changes")
	sock := fs.String("socket", server.DefaultSocketPath(), "unix socket path")
	// allow "set insert --ttl 5s" as well as "set --ttl 5s insert"
	var mode string
	rest := args
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		mode, rest = rest[0], rest[1:]
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if mode == "" && fs.NArg() > 0 {
		mode = fs.Arg(0)
	}
	if mode == "" {
		return errors.New("usage: zmk-vim-mode set <off|normal|insert|visual|cmdline|raw|legacy|auto> [--ttl 30s] [--sticky]")
	}
	reply, err := server.Request(*sock, proto.Msg{V: proto.Version, T: proto.TSet, Mode: mode,
		TTLMs: int(ttl.Milliseconds()), Sticky: *sticky}, 2*time.Second)
	if err != nil {
		return err
	}
	if reply.T == proto.TError {
		return fmt.Errorf("%s: %s", reply.Err, reply.Message)
	}
	fmt.Printf("code=%d mode=%s (%s)\n", deref(reply.Code), reply.Mode, reply.Reason)
	return nil
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print raw JSON")
	sock := fs.String("socket", server.DefaultSocketPath(), "unix socket path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	reply, err := server.Request(*sock, proto.Msg{V: proto.Version, T: proto.TStatus}, 2*time.Second)
	if err != nil {
		return err
	}
	if reply.Status == nil {
		return errors.New("malformed status reply")
	}
	st := reply.Status
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(st)
	}
	if st.Version != "" && st.Version != Version {
		fmt.Printf("daemon   : %s (this CLI is %s — restart the service after make install)\n", st.Version, Version)
	}
	fmt.Printf("decision : %s (code %d) — %s\n", st.Mode, st.Code, st.Reason)
	if st.Frontmost != nil {
		if st.Frontmost.Known {
			fmt.Printf("frontmost: %s", st.Frontmost.Class)
			if st.Frontmost.PID != 0 {
				fmt.Printf(" (pid %d)", st.Frontmost.PID)
			}
			fmt.Println()
		} else {
			fmt.Println("frontmost: unknown (no focus backend) — trusting clients")
		}
	}
	if st.Widget != nil {
		where := "text editor"
		if !st.Widget.Editor {
			where = "elsewhere"
		}
		fmt.Printf("focus    : %s (%s, via the accessibility bus)\n", where, st.Widget.Detail)
	}
	if st.Override != nil {
		fmt.Printf("override : %s", st.Override.Mode)
		if st.Override.LeftMs > 0 {
			fmt.Printf(" (%s left)", (time.Duration(st.Override.LeftMs) * time.Millisecond).Round(time.Second))
		}
		if st.Override.Sticky {
			fmt.Print(" sticky")
		}
		fmt.Println()
	}
	fmt.Printf("clients  : %d\n", len(st.Clients))
	for _, c := range st.Clients {
		foc := "focus=?"
		if c.Focused != nil {
			foc = fmt.Sprintf("focus=%v", *c.Focused)
		}
		extra := ""
		if c.App != "" {
			extra += " app=" + c.App
		}
		if c.Nested {
			extra += " nested"
		}
		if c.TTLLeftMs != 0 {
			extra += fmt.Sprintf(" ttl=%dms", c.TTLLeftMs)
		}
		fmt.Printf("  #%d %s pid=%d mode=%s %s idle=%dms%s\n", c.ID, c.Kind, c.PID, c.Mode, foc, c.IdleMs, extra)
	}
	fmt.Printf("devices  : %d\n", len(st.Devices))
	printDevices(st.Devices)
	fmt.Printf("uptime   : %s\n", (time.Duration(st.UptimeS) * time.Second))
	return nil
}

func runDevices(args []string) error {
	fs := flag.NewFlagSet("devices", flag.ContinueOnError)
	sock := fs.String("socket", server.DefaultSocketPath(), "unix socket path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	reply, err := server.Request(*sock, proto.Msg{V: proto.Version, T: proto.TDevices}, 2*time.Second)
	if err != nil {
		return err
	}
	if len(reply.Devices) == 0 {
		fmt.Println("no keyboards with Compose+Kana+Scroll LEDs found (is CONFIG_ZMK_HID_INDICATORS=y? udev rule installed?)")
		return nil
	}
	printDevices(reply.Devices)
	return nil
}

func printDevices(devs []proto.Device) {
	for _, d := range devs {
		last := "-"
		if d.LastCode != nil {
			last = fmt.Sprintf("%d (%s)", *d.LastCode, state.ModeName(*d.LastCode))
		}
		w := "writable"
		if !d.Writable {
			w = "NOT writable"
		}
		fmt.Printf("  %s  %s  %s  %04x:%04x  last=%s  %s", d.ID, d.Product, d.Transport, d.VID, d.PID, last, w)
		if d.Note != "" {
			fmt.Printf("  [%s]", d.Note)
		}
		fmt.Println()
	}
}

func runInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	nvim := fs.Bool("nvim", false, "print the lazy.nvim plugin spec")
	tmux := fs.Bool("tmux", false, "print the tmux focus-events line")
	udev := fs.Bool("udev", false, "print the udev rule (Linux)")
	noService := fs.Bool("no-service", false, "do not install/enable the user service")
	vscode := fs.Bool("vscode", false, "apply the VSCode settings the daemon relies on and install the companion extension + vscode-neovim via the `code` CLI")
	atspiOn := fs.Bool("atspi", false, "start the daemon with --atspi (follow focus inside VSCode through the accessibility bus)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return install.Run(os.Stdout, install.Options{Nvim: *nvim, Tmux: *tmux, Udev: *udev, VSCode: *vscode, ATSPI: *atspiOn, Service: !*noService, Version: Version})
}

func deref(p *uint8) uint8 {
	if p == nil {
		return 0
	}
	return *p
}
