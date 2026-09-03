// Package install writes the per-user service unit and prints the snippets the
// user must add themselves (editor spec, tmux option, udev rule). It never
// edits the user's dotfiles.
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
ExecStart=%s daemon
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
			content = fmt.Sprintf(systemdUnit, exe)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(w, "wrote %s\n", path)
		fmt.Fprintln(w, "\nenable it with:")
		if runtime.GOOS == "darwin" {
			fmt.Fprintf(w, "  launchctl bootstrap gui/$UID %s\n", path)
		} else {
			fmt.Fprintln(w, "  systemctl --user daemon-reload")
			fmt.Fprintln(w, "  systemctl --user enable --now zmk-vim-mode.service")
		}
	}
	if o.Udev {
		fmt.Fprintln(w, "\n--- /etc/udev/rules.d/60-zmk-vim-mode.rules (needs sudo) ---")
		fmt.Fprint(w, UdevRule)
	}
	if o.Nvim {
		fmt.Fprintln(w, "\n--- Neovim (lazy.nvim) ---")
		fmt.Fprint(w, NvimSpec)
	}
	if o.Tmux {
		fmt.Fprintln(w, "\n--- tmux ---")
		fmt.Fprint(w, TmuxSnippet)
	}
	if !o.Nvim && !o.Tmux && !o.Udev {
		fmt.Fprintln(w, "\nrun with --nvim --tmux --udev to print the editor, tmux and udev snippets.")
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
