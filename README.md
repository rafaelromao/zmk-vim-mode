# zmk-vim-mode

Keeps a ZMK keyboard's layers in sync with the editor's real vim state, so the
keyboard is in NORMAL when the editor is in normal mode, in INSERT when you are
typing, and out of vim layers entirely when keys must pass through untouched.

Three parts, one repository:

| Part | Path | Role |
|---|---|---|
| Host daemon | `cmd/`, `internal/` | decides the state, writes it to the keyboard |
| Neovim plugin | `lua/`, `plugin/` | reports Neovim's effective state over a unix socket |
| ZMK module | `firmware/`, `zephyr/` | decodes the state and switches layers |

See [PLAN.md](PLAN.md) for the full design and the reasoning behind it.

## How it works

The host encodes the editor state as a 3-bit code and writes it into the
keyboard's HID LED output report, using the three indicators no operating
system drives on its own: **Compose**, **Kana** and **Scroll Lock**. This is
the only channel that works identically over USB and Bluetooth with stock ZMK.
Caps Lock and Num Lock are never touched.

| code | state | keyboard |
|---|---|---|
| 0 | off | no vim layers |
| 1 | normal | `VIM_NORMAL` |
| 2 | insert | `VIM_INSERT` (Neovim's Replace maps here too) |
| 3 | visual | `VIM_NORMAL` + `VIM_VISUAL` |
| 4 | legacy | vim-like app with no mode feed; the keyboard infers modes itself |
| 5 | cmdline | `VIM_CMDLINE` |
| 6 | raw | no vim layers: keys pass through untouched |
| 7 | legacy silent | same state as 4, re-asserted without re-injecting Esc |

**Raw** is the state that ad-hoc focus watchers cannot produce. It is reported
for plugin UI buffers whose letters are commands (dashboard, file explorer,
lazy, mason, trouble), for terminal-job mode, and while a `<leader>` sequence
is pending. Without it, the keyboard's NORMAL layer would remap those letters
and the UI would be unusable.

The keyboard keeps its own local inference, which the host only corrects. A
firmware guard parks host codes for 150 ms after any keyboard-driven layer
change, so a stale code cannot undo a faster local transition; and code 0 is
held off for 60 ms, so the LED clobber Linux compositors cause is invisible.

## Install (Omarchy / Hyprland)

```bash
make install                     # builds, installs the binary and the user service
sudo cp contrib/udev/60-zmk-vim-mode.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules && sudo udevadm trigger
systemctl --user daemon-reload && systemctl --user enable --now zmk-vim-mode.service
```

Add the Neovim plugin (`contrib/nvim-lazy-spec.lua` → `~/.config/nvim/lua/plugins/`),
put `set -g focus-events on` in `~/.tmux.conf`, then flash the firmware module
following [docs/keyboards-repo.md](docs/keyboards-repo.md).

Check everything with `zmk-vim-mode doctor`.

macOS is not wired up yet: the daemon runs with a logging-only LED backend, so
the socket protocol and the plugin work, but nothing reaches the keyboard. See
PLAN.md phase 6.

## Commands

```
zmk-vim-mode daemon      run the daemon (normally via the user service)
zmk-vim-mode status      current decision, frontmost app, clients, devices
zmk-vim-mode devices     keyboards the daemon can write to
zmk-vim-mode set <mode>  manual override; repeating the same mode returns to auto
zmk-vim-mode doctor      check permissions, devices, old watchers, tmux, udev
```

`set` is the escape hatch for anything the daemon cannot detect (an SSH
session, a screen-sharing app). Bind it to a key with
`contrib/hyprland-bind.conf`.

## Development

```bash
make test          # Go, Neovim and firmware-policy suites
make lint          # gofmt + go vet, for this platform and for Linux
make cross         # binaries for the Omarchy box
```

`scripts/spike-linux.sh` covers the bring-up checks on Linux: find the
keyboard's device nodes, confirm the firmware exposes the three LEDs, write a
code by hand, and watch the `EV_LED` echoes that reveal a clobber.

The firmware's decode and timing rules live in `firmware/src/code_policy.h`,
which depends on nothing but `stdint`, so they are unit-tested on the host
(`make test-firmware`) rather than only on hardware.

## Licence

MIT. See [LICENSE](LICENSE).
