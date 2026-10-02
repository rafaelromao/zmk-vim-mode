// Package install writes the per-user service unit and prints the snippets the
// user must add themselves (editor spec, tmux option, udev rule). It edits a
// user file only when asked to explicitly (--vscode), and keeps a backup.
package install

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Options selects what to install/print.
type Options struct {
	Service bool
	Nvim    bool
	Tmux    bool
	Udev    bool
	// VSCode applies the settings the daemon relies on and installs the
	// companion extension and vscode-neovim through the `code` CLI.
	VSCode bool
	// Obsidian copies the plugin into every registered vault and enables it.
	Obsidian bool
	// IntelliJ builds the plugin against each installed JetBrains IDE and
	// unpacks it where that IDE loads plugins from.
	IntelliJ bool
	// Hammerspoon installs the menu bar indicator as a Spoon and has init.lua
	// start it (macOS).
	Hammerspoon bool
	// Omarchy installs the bar widget plugin into Omarchy's Quattro shell and
	// puts it in the bar (Linux).
	Omarchy bool
	// ATSPI starts the service with --atspi.
	ATSPI bool
	// PathEntry appends the binary's directory to the login shell's profile
	// when it is not already on PATH.
	PathEntry bool
	// OpenPrivacy opens the macOS panes whose grants cannot be scripted, so
	// the user lands on the right list instead of hunting for it.
	OpenPrivacy bool
	Version     string
}

// UdevRule is the rule granting the seat user read/write on the ZMK keyboard's
// hidraw and evdev nodes. It must sort before systemd's 73-seat-late.rules.
const UdevRule = `# zmk-vim-mode: let the active seat user write LED output reports to the ZMK keyboard.
# Install as /etc/udev/rules.d/60-zmk-vim-mode.rules, then:
#   sudo udevadm control --reload-rules && sudo udevadm trigger
# uaccess grants an ACL to the user of the active logind session. If that does
# not fit (headless, SSH-only), replace TAG+="uaccess" with GROUP="input", MODE="0660".
# bus 0003 = USB, 0005 = Bluetooth; this form needs no ancestor attributes.
SUBSYSTEM=="hidraw", KERNELS=="0003:1D50:615E.*", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNELS=="0005:1D50:615E.*", TAG+="uaccess"
SUBSYSTEM=="hidraw", ATTRS{idVendor}=="1d50", ATTRS{idProduct}=="615e", TAG+="uaccess"
SUBSYSTEM=="input", KERNEL=="event*", ATTRS{id/vendor}=="1d50", ATTRS{id/product}=="615e", TAG+="uaccess"
`

// NvimSpec is the lazy.nvim plugin spec.
const NvimSpec = `-- ~/.config/nvim/lua/plugins/zmk-vim-mode.lua
return {
  {
    "rafaelromao/zmk-vim-mode",
    -- while developing: dir = vim.fn.expand("~/projects/zmk-vim-mode"),
    lazy = false,
    vscode = true, -- LazyVim disables every other plugin inside vscode-neovim
    opts = {},
  },
}
`

// TmuxSnippet enables focus events (tmux defaults them off).
const TmuxSnippet = `# ~/.tmux.conf — required so Neovim sees FocusGained/FocusLost inside tmux
set -g focus-events on
`

const systemdUnit = `[Unit]
Description=zmk-vim-mode: sync ZMK keyboard layers with the editor's vim state
After=graphical-session.target
PartOf=graphical-session.target

[Service]
Type=simple
ExecStart=%s daemon%s
Restart=always
RestartSec=2

[Install]
WantedBy=graphical-session.target
`

const launchdPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>dev.rafaelromao.zmk-vim-mode</string>
  <key>ProgramArguments</key><array><string>%s</string><string>daemon</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>ProcessType</key><string>Interactive</string>
  <key>LimitLoadToSessionType</key><string>Aqua</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict>
</plist>
`

// systemdArg makes p a single word of a unit's command line. systemd expands
// % (specifiers) and $ (variables) anywhere on the line, so both are doubled;
// anything that splits or quotes a word, or starts a C-style escape, puts the
// word in double quotes, where those escapes are applied.
func systemdArg(p string) string {
	esc := strings.NewReplacer(`%`, `%%`, `$`, `$$`).Replace(p)
	if !strings.ContainsAny(p, " \t\n\"'\\") {
		return esc
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`).Replace(esc) + `"`
}

// xmlText escapes s for the text of a plist <string>.
func xmlText(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// ServicePath returns the path of the user service file for this platform.
func ServicePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "LaunchAgents", AgentLabel+".plist"), nil
	}
	return filepath.Join(home, ".config", "systemd", "user", UnitName), nil
}

// Run performs the installation.
func Run(w io.Writer, o Options) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	if o.PathEntry {
		if err := EnsurePATH(w, filepath.Dir(exe)); err != nil {
			// A profile we cannot write is not worth failing the install for:
			// everything else still works, and the daemon runs by full path.
			fmt.Fprintf(w, "path       : could not update your shell profile (%v)\n", err)
		}
	}
	if o.Service {
		path, err := ServicePath()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		var content string
		if runtime.GOOS == "darwin" {
			home, _ := os.UserHomeDir()
			content = fmt.Sprintf(launchdPlist, xmlText(exe), xmlText(filepath.Join(home, "Library", "Logs", "zmk-vim-mode.log")))
		} else {
			// --atspi is sticky: a plain `make install` must not silently turn it
			// off. Uninstall, then install without it, to drop it.
			atspi := o.ATSPI
			if old, err := os.ReadFile(path); err == nil && strings.Contains(string(old), "--atspi") {
				if !atspi {
					fmt.Fprintln(w, "keeping --atspi from the existing unit")
				}
				atspi = true
			}
			extra := ""
			if atspi {
				extra = " --atspi"
			}
			content = fmt.Sprintf(systemdUnit, systemdArg(exe), extra)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(w, "wrote %s\n", path)
		if runtime.GOOS == "darwin" {
			// Restart a running agent ourselves: launchd keeps the old binary
			// image otherwise, and on macOS that also means the old process
			// identity, so a re-granted Input Monitoring permission would not
			// apply to it either.
			// The agent is unloaded and loaded again, as `zmk-vim-mode
			// restart` does: launchd has refused to kickstart a rebuilt
			// binary, and loading also makes it read the plist written above.
			s := &service{goos: "darwin", plist: path, run: runCommand, sleep: time.Sleep}
			if wasLoaded, err := s.reload(); err != nil {
				fmt.Fprintf(w, "\ncould not restart the agent (%v); run: zmk-vim-mode restart\n", err)
			} else if wasLoaded {
				fmt.Fprintln(w, "restarted the agent (it now runs the new binary)")
			} else {
				fmt.Fprintln(w, "loaded the agent")
			}
			// Input Monitoring and Accessibility live in TCC, which is
			// SIP-protected: nothing outside System Settings may grant them,
			// so opening the right pane is as far as automation goes.
			fmt.Fprintln(w, "\nmacOS ties Input Monitoring and Accessibility to the binary's identity,")
			fmt.Fprintln(w, "so a rebuilt daemon can lose either. In each pane, remove the old")
			fmt.Fprintln(w, "zmk-vim-mode entry with − and add it again:")
			fmt.Fprintf(w, "  %s\n", exe)
			if o.OpenPrivacy {
				openPrivacyPanes(w)
			} else {
				fmt.Fprintln(w, "  System Settings → Privacy & Security → Input Monitoring, and → Accessibility")
			}
			fmt.Fprintln(w, "then restart the daemon so it picks them up. It lists the keyboards when it")
			fmt.Fprintln(w, "is back, and `writable` means Input Monitoring took effect:")
			fmt.Fprintln(w, "  zmk-vim-mode restart")
		} else {
			// Rewriting the unit without telling systemd leaves it acting on a
			// stale copy, and its warning is easy to miss in build output.
			if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
				fmt.Fprintf(w, "\ncould not run systemctl --user daemon-reload (%v); run it yourself\n", err)
			} else {
				fmt.Fprintln(w, "ran systemctl --user daemon-reload")
			}
			// Same reason as on macOS: a running unit keeps the old binary
			// until it is restarted, and a stale daemon is hard to spot.
			if exec.Command("systemctl", "--user", "is-active", "--quiet", UnitName).Run() == nil {
				if err := exec.Command("systemctl", "--user", "restart", UnitName).Run(); err != nil {
					fmt.Fprintf(w, "\ncould not restart the service (%v); run: zmk-vim-mode restart\n", err)
				} else {
					fmt.Fprintln(w, "restarted the service (it now runs the new binary)")
				}
			} else {
				// enable as well as start, so it also comes up at login.
				fmt.Fprintln(w, "\nstart it, now and at every login, with:")
				fmt.Fprintln(w, "  systemctl --user enable --now "+UnitName)
			}
		}
	}
	// udev is Linux's device-permission mechanism; on macOS the equivalent is
	// the Input Monitoring grant printed above.
	if o.Udev && runtime.GOOS == "linux" {
		fmt.Fprintln(w, "\n--- /etc/udev/rules.d/60-zmk-vim-mode.rules (needs sudo) ---")
		fmt.Fprint(w, UdevRule)
	}
	if o.Nvim {
		fmt.Fprintln(w, "\n--- Neovim (lazy.nvim) ---")
		if err := InstallNvim(w); err != nil {
			return err
		}
	}
	if o.Tmux {
		fmt.Fprintln(w, "\n--- tmux ---")
		fmt.Fprint(w, TmuxSnippet)
	}
	if o.VSCode {
		fmt.Fprintln(w, "\n--- VSCode ---")
		atspi := o.ATSPI
		if p, err := ServicePath(); err == nil {
			if old, err := os.ReadFile(p); err == nil && strings.Contains(string(old), "--atspi") {
				atspi = true
			}
		}
		if err := InstallVSCode(w, atspi); err != nil {
			return err
		}
	}
	if o.Obsidian {
		fmt.Fprintln(w, "\n--- Obsidian ---")
		if err := InstallObsidian(w); err != nil {
			return err
		}
	}
	if o.IntelliJ {
		fmt.Fprintln(w, "\n--- IntelliJ ---")
		if err := InstallIntelliJ(w); err != nil {
			return err
		}
	}
	// The status bar indicators come last and do not fail the install: they
	// are a convenience, and `make install` still has the udev rule and the
	// service to set up after this returns.
	if o.Hammerspoon {
		fmt.Fprintln(w, "\n--- Hammerspoon (menu bar) ---")
		if err := InstallHammerspoon(w, exe); err != nil {
			fmt.Fprintf(w, "hammerspoon: %v\n", err)
		}
	}
	if o.Omarchy {
		fmt.Fprintln(w, "\n--- Omarchy (bar widget) ---")
		if err := InstallOmarchy(w, exe); err != nil {
			fmt.Fprintf(w, "omarchy    : %v\n", err)
		}
	}
	if !o.Nvim && !o.Tmux && !o.Udev && !o.VSCode && !o.Obsidian && !o.IntelliJ && !o.Hammerspoon && !o.Omarchy {
		fmt.Fprintln(w, "\nrun with --nvim --tmux --udev to print the editor, tmux and udev snippets,")
		fmt.Fprintln(w, "--vscode to apply the VSCode settings and install the companion extension,")
		fmt.Fprintln(w, "--obsidian to install the plugin into your vaults,")
		fmt.Fprintln(w, "--intellij to build and install the plugin for your JetBrains IDEs,")
		fmt.Fprintln(w, "--hammerspoon (macOS) or --omarchy (Omarchy Quattro) to install the status bar indicator.")
	}
	return nil
}

// Uninstall removes the service file (leaving user config untouched).
func Uninstall(w io.Writer) error {
	path, err := ServicePath()
	if err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		_ = exec.Command("launchctl", "bootout", agentTarget()).Run()
	} else {
		_ = exec.Command("systemctl", "--user", "disable", "--now", UnitName).Run()
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Fprintf(w, "removed %s\n", path)
	fmt.Fprintln(w, "the udev rule (if installed) must be removed manually: sudo rm /etc/udev/rules.d/60-zmk-vim-mode.rules")
	return nil
}

// TrimHome shortens a path for display.
func TrimHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	return strings.Replace(p, home, "~", 1)
}
