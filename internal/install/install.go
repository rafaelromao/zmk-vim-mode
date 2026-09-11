// Package install writes the per-user service unit and prints the snippets the
// user must add themselves (editor spec, tmux option, udev rule). It edits a
// user file only when asked to explicitly (--vscode), and keeps a backup.
package install

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	// ATSPI starts the service with --atspi.
	ATSPI   bool
	Version string
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

// ServicePath returns the path of the user service file for this platform.
func ServicePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "LaunchAgents", "dev.rafaelromao.zmk-vim-mode.plist"), nil
	}
	return filepath.Join(home, ".config", "systemd", "user", "zmk-vim-mode.service"), nil
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
			content = fmt.Sprintf(launchdPlist, exe, filepath.Join(home, "Library", "Logs", "zmk-vim-mode.log"))
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
			content = fmt.Sprintf(systemdUnit, exe, extra)
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
			label := fmt.Sprintf("gui/%d/dev.rafaelromao.zmk-vim-mode", os.Getuid())
			if exec.Command("launchctl", "print", label).Run() == nil {
				if err := exec.Command("launchctl", "kickstart", "-k", label).Run(); err != nil {
					fmt.Fprintf(w, "\ncould not restart the agent (%v); run it yourself:\n  launchctl kickstart -k %s\n", err, label)
				} else {
					fmt.Fprintln(w, "restarted the agent (it now runs the new binary)")
				}
			} else {
				fmt.Fprintln(w, "\nenable it with:")
				fmt.Fprintf(w, "  launchctl bootstrap gui/$UID %s\n", path)
			}
			fmt.Fprintln(w, "\nmacOS ties Input Monitoring to the binary's identity, so a rebuilt daemon loses it:")
			fmt.Fprintln(w, "  System Settings → Privacy & Security → Input Monitoring → remove zmk-vim-mode, add it again")
			fmt.Fprintf(w, "  (the binary is %s)\n", TrimHome(exe))
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
			if exec.Command("systemctl", "--user", "is-active", "--quiet", "zmk-vim-mode.service").Run() == nil {
				if err := exec.Command("systemctl", "--user", "restart", "zmk-vim-mode.service").Run(); err != nil {
					fmt.Fprintf(w, "\ncould not restart the service (%v); run: systemctl --user restart zmk-vim-mode.service\n", err)
				} else {
					fmt.Fprintln(w, "restarted the service (it now runs the new binary)")
				}
			} else {
				fmt.Fprintln(w, "\nstart it with:")
				fmt.Fprintln(w, "  systemctl --user enable --now zmk-vim-mode.service")
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
	if !o.Nvim && !o.Tmux && !o.Udev && !o.VSCode && !o.Obsidian {
		fmt.Fprintln(w, "\nrun with --nvim --tmux --udev to print the editor, tmux and udev snippets,")
		fmt.Fprintln(w, "--vscode to apply the VSCode settings and install the companion extension,")
		fmt.Fprintln(w, "--obsidian to install the plugin into your vaults.")
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
		_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/dev.rafaelromao.zmk-vim-mode", os.Getuid())).Run()
	} else {
		_ = exec.Command("systemctl", "--user", "disable", "--now", "zmk-vim-mode.service").Run()
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Fprintf(w, "removed %s\n", path)
	fmt.Fprintln(w, "the udev rule (if installed) must be removed manually: sudo rm /etc/udev/rules.d/60-zmk-vim-mode.rules")
	return nil
}

// OldWatchers lists the files of the previous ad-hoc solution, so doctor can
// tell the user what to retire.
func OldWatchers() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".hammerspoon", "zmk-vim-mode-watcher.lua"),
		filepath.Join(home, ".hammerspoon", "zmk-vim-mode.sh"),
		filepath.Join(home, ".config", "hypr", "zmk-vim-mode-watcher.sh"),
		filepath.Join(home, ".config", "hypr", "zmk-vim-mode.sh"),
		filepath.Join(home, ".config", "systemd", "user", "zmk-vim-mode-watcher.service"),
	}
}

// TrimHome shortens a path for display.
func TrimHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	return strings.Replace(p, home, "~", 1)
}
