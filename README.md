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

### The channel

A HID keyboard has an *output* report: a byte the host sends **to** the
keyboard, normally to light the Caps Lock and Num Lock lamps. It is the only
standard host-to-keyboard channel that works identically over USB and Bluetooth
with stock ZMK, needs no pairing or custom protocol, and requires nothing of
the firmware beyond `CONFIG_ZMK_HID_INDICATORS=y`. This project uses it as a
data bus rather than as lamp control.

ZMK declares five indicator bits. You can confirm this in your own keyboard's
HID report descriptor, where `19 01 29 05` means "usage minimum NumLock,
usage maximum Kana":

```
05 08  19 01  29 05  75 01  95 05  91 02
│      │      │      │      │      └─ output: data, variable, absolute
│      │      │      │      └──────── five of them
│      │      │      └─────────────── one bit each
│      │      └────────────────────── usage max = Kana      (0x05)
│      └───────────────────────────── usage min = Num Lock  (0x01)
└──────────────────────────────────── usage page = LED
```

Three of those five are free: no operating system ever *sets* **Compose**,
**Kana** or **Scroll Lock** on its own. Num Lock and Caps Lock are deliberately
left alone, because the OS owns them — a stray lock keypress would otherwise
change your editor state. Those three free bits are read as one 3-bit number.

### The codes on the wire

| code | state | Scroll `0x04` | Kana `0x10` | Compose `0x08` | byte | keyboard |
|---|---|---|---|---|---|---|
| 0 | off | · | · | · | `0x00` | no vim layers |
| 1 | normal | · | · | ● | `0x08` | `VIM_NORMAL` |
| 2 | insert | · | ● | · | `0x10` | `VIM_INSERT` (Neovim's Replace maps here too) |
| 3 | visual | · | ● | ● | `0x18` | `VIM_NORMAL` + `VIM_VISUAL` |
| 4 | legacy | ● | · | · | `0x04` | vim-like app with no mode feed; the keyboard infers modes itself |
| 5 | cmdline | ● | · | ● | `0x0c` | `VIM_CMDLINE` |
| 6 | raw | ● | ● | · | `0x14` | no vim layers: keys pass through untouched |
| 7 | legacy silent | ● | ● | ● | `0x1c` | same state as 4, re-asserted without re-injecting Esc |

The lamps stay dark: unless the keymap has a `zmk,indicator-leds` node, nothing
physically lights up. It is a silent side channel that happens to travel on the
LED wire.

### End to end, pressing `i` in Neovim

```
nvim ModeChanged ──► plugin ──► unix socket ──► daemon
                                                  │ decides code 2 (insert)
                                                  ▼
                              write [0x01, 0x10] to /dev/hidrawN
                                                  │  report id, LED byte
                     kernel ──► USB SET_REPORT  /  BLE GATT write
                                                  ▼
                     ZMK zmk_hid_indicators_process_report()
                                                  ▼
                       event: zmk_hid_indicators_changed
                                                  ▼
                       this module: decode 0x10 → code 2
                                                  ▼
             deactivate the managed layers, activate VIM_INSERT
```

The firmware side does no scanning and types no keys: it tests three bits,
rebuilds the integer, and if it differs from the last one, switches the layer
set. That is a bitmask change, so it is effectively instant.

Writes go to **every** matching endpoint. A keyboard commonly enumerates on USB
and Bluetooth at the same time, and ZMK stores the indicator byte per endpoint
while only the selected one raises the event, so writing just one is a coin
flip.

### Raw, the state focus watchers cannot produce

**Raw** is reported for plugin UI buffers whose letters are commands
(dashboard, file explorer, lazy, mason, trouble), for terminal-job mode, and
while a `<leader>` sequence is pending. Without it, the keyboard's NORMAL layer
would remap those letters and the UI would be unusable. It is deliberately
distinct from insert: insert keeps the keyboard's local inference alive, so Esc
would flip it to normal — wrong when Esc belongs to a terminal job or a picker.

### Two timing rules that make it survive real use

The keyboard keeps its own local inference and the host only *corrects* it.
Two rules in the firmware keep that hybrid honest:

- **OFF hold-off, 60 ms.** When the kernel sends its own LED report — you
  pressed Num Lock, or a compositor pushed lock state — it sends the whole
  byte, zeroing our bits. Code 0 is therefore applied only after it has been
  stable for 60 ms. The daemon sees the `EV_LED` echo and rewrites within about
  a millisecond, so the blip never reaches your layers.
- **Local guard, 150 ms.** A host code describes the editor as of an earlier
  keystroke. Type `Esc` then `i` quickly and an in-flight stale code could undo
  a correct local transition, so after any keyboard-driven layer change host
  codes are parked and only the newest is applied. The keyboard wins in motion,
  the host wins at rest.

On Linux the steady state is quiet: typing produces no LED reports at all,
because the kernel's cache holds 0 for Compose and Kana, so a compositor
writing "all five bits" changes nothing and is dropped before any report is
emitted. Re-assertion happens only on a real clobber or a reconnect.

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

### Editors

| Editor | Mode source | Tool-window focus | Setup |
|---|---|---|---|
| Neovim in a terminal, Neovide | the Neovim plugin | the plugin: `raw` for pickers, the terminal, a pending `<leader>` | `contrib/nvim-lazy-spec.lua` |
| VSCode | the same plugin, inside [vscode-neovim](https://github.com/vscode-neovim/vscode-neovim) | window title `[${focusedView}]` read from Hyprland, plus a small companion extension for quick inputs and non-text editors | `zmk-vim-mode install --vscode`, then [editors/vscode](editors/vscode/README.md) |
| Obsidian | own plugin (CodeMirror vim events) | own plugin (`focusin`) | `zmk-vim-mode install --obsidian`, then [editors/obsidian](editors/obsidian/README.md) |
| IntelliJ, anything else | none: `legacy`, the keyboard infers | — | nothing; `set raw` when a tool window traps you |

Inside an app the sources rank: window title (a focused tool window) →
accessibility bus (focus anywhere but the text editor) → the app's own client
saying `raw` → the best client with a real mode → `legacy`. The title wins
because no client can see focus leave the text editor.

### Following focus through the accessibility bus (optional)

Two things nothing above can see: a quick input opened with the mouse, and the
exact moment focus returns to the editor. Both are visible on AT-SPI2, the
Linux accessibility bus, which every toolkit reports focus changes to -- when
accessibility is on.

```bash
zmk-vim-mode install --atspi --vscode && systemctl --user restart zmk-vim-mode   # then restart VSCode once
```

`--vscode` alongside `--atspi` also adds `--force-renderer-accessibility` to
`~/.config/code-flags.conf` (read by Arch's `code` wrapper): Electron builds
the accessibility tree of its web content only with that switch -- the bus
flags alone reach GTK and Qt, not VSCode's DOM.

The daemon then keeps one connection to the bus, registers as a focus
listener and classifies each focused widget of the frontmost VSCode: the
Monaco code editor → the clients decide; anything else (the palette's input,
the terminal, a tree, a rename box, the find widget) → raw. It also tells the
VSCode companion and the embedded Neovim when the editor regained focus, so
their own guesses clear instantly.

What it costs, and why it is opt-in: the daemon sets `org.a11y.Status.IsEnabled`
on the session, which is what a screen reader does -- GTK, Qt and Chromium
applications start maintaining accessibility trees (a little CPU and memory,
nothing visible). `install --vscode` already sets
`editor.accessibilitySupport: off` so VSCode does not switch Monaco into
screen-reader mode because of it. Applications read the flag at startup:
restart VSCode after enabling. The daemon reads only roles, labels and HTML
tag/class names of focused widgets -- never text -- and logs labels at debug
only.

`zmk-vim-mode atspi-watch` prints every focus event with the classifier's
verdict; if a VSCode update renames a widget, that is where the new name shows
up. `zmk-vim-mode status` shows `focus : elsewhere (input.input …)` while a
quick input is open. No D-Bus library is involved: `internal/dbus` is a
300-line client for the handful of calls this needs.

macOS is not wired up yet: the daemon runs with a logging-only LED backend, so
the socket protocol and the plugin work, but nothing reaches the keyboard. See
PLAN.md phase 6.

## Commands

```
zmk-vim-mode daemon      run the daemon (normally via the user service)
zmk-vim-mode status      current decision, frontmost app, clients, devices
zmk-vim-mode devices     keyboards the daemon can write to
zmk-vim-mode set <mode>  manual override; repeating the same mode returns to auto
zmk-vim-mode doctor      check the daemon, devices, udev, tmux, old watchers, and the VSCode/Obsidian setups
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
