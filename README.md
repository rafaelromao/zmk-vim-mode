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
make install                     # builds, installs the binary, writes the service and the Neovim spec
sudo cp contrib/udev/60-zmk-vim-mode.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules && sudo udevadm trigger
systemctl --user enable --now zmk-vim-mode.service
```

`make install` writes the lazy.nvim spec to `~/.config/nvim/lua/plugins/` when
your config has none (an existing one is never touched), runs
`systemctl --user daemon-reload`, and restarts the service if it is already
running — a service left on the old binary is the classic reason a change
seems to do nothing. Add `set -g focus-events on` to `~/.tmux.conf`, then flash
the firmware module following [docs/keyboards-repo.md](docs/keyboards-repo.md).

Editors other than Neovim are one more command, then a restart of each:

```bash
zmk-vim-mode install --vscode --obsidian --atspi && systemctl --user restart zmk-vim-mode
```

Check everything with `zmk-vim-mode doctor`.

### Editors

| Editor | Mode source | Tool-window focus | Setup |
|---|---|---|---|
| Neovim in a terminal, Neovide | the Neovim plugin | the plugin: `raw` for pickers, the terminal, a pending `<leader>` | `zmk-vim-mode install --nvim` (also done by `make install`) |
| VSCode | the same plugin, inside [vscode-neovim](https://github.com/vscode-neovim/vscode-neovim) | window title `[${focusedView}]` read from Hyprland, a companion extension for quick inputs and non-text editors, the accessibility bus for anything opened with the mouse (Linux) | `zmk-vim-mode install --vscode --atspi`, then [editors/vscode](editors/vscode/README.md) |
| Obsidian | own plugin (CodeMirror vim events) | own plugin (`focusin`) | `zmk-vim-mode install --obsidian`, then [editors/obsidian](editors/obsidian/README.md) |
| IntelliJ, anything else | none: `legacy`, the keyboard infers | — | nothing; `set raw` when a tool window traps you |

Inside an app the sources rank: window title (a focused tool window) →
accessibility bus (focus anywhere but the text editor) → the app's own client
saying `raw` → the best client with a real mode → `legacy`. The title wins
because no client can see focus leave the text editor.

#### Neovim plugin options

Passed as `opts` in the lazy.nvim spec; all have working defaults.

| Option | Default | Meaning |
|---|---|---|
| `socket` | `~/.local/state/zmk-vim-mode/daemon.sock` | daemon socket; `$ZMK_VIM_MODE_SOCKET` also works |
| `terminal_state` | `raw` | what `:terminal` job mode reports — `insert` keeps the vim layers there |
| `leader_raw` | `true` | report `raw` while a `<leader>` sequence is pending (which-key menus) |
| `raw_filetypes` | see `lua/zmk-vim-mode/context.lua` | filetypes whose single letters are plugin commands |
| `raw_exceptions` | `help qf man checkhealth` | filetypes that stay in vim layers despite looking like UI |
| `classify` | `nil` | `function(buf, win, mode) -> state\|nil`, overrides everything |
| `vscode_raw_actions` | palette, Go to File/Line/Symbol, rename | VSCode commands (Lua patterns) that take the keys away from Neovim |
| `vscode_raw_ttl_ms` | `20000` | how long that hint lasts if its close is never observed |
| `debug` | `false` | log every state change with `vim.notify` |

`:ZmkVimMode status` shows the socket, the connection, the state last sent and
whether a leader or VSCode hint is active.

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

What it costs, and why it is opt-in: the daemon sets both `org.a11y.Status`
flags (`IsEnabled`, `ScreenReaderEnabled`) on the session, which is what a
screen reader does -- GTK, Qt and Chromium applications start maintaining
accessibility trees (a little CPU and memory, nothing visible). `install --vscode` already sets
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

## Install (macOS)

```bash
make install                                                        # cgo build, signs the binary, writes the agent
launchctl bootstrap gui/$UID ~/Library/LaunchAgents/dev.rafaelromao.zmk-vim-mode.plist
```

`bootstrap` is only for loading the agent the first time; afterwards
`make install` restarts it itself (`launchctl kickstart -k` if you need to do
it by hand — `bootstrap` on a loaded agent fails with `5: Input/output error`).

Then grant **Input Monitoring** to `~/.local/bin/zmk-vim-mode`: System Settings
→ Privacy & Security → Input Monitoring → `+`, ⌘⇧G to type the path. Opening a
keyboard's HID device requires it, and a background agent is never prompted, so
the entry has to be added by hand. `zmk-vim-mode devices` tells you whether it
took: every keyboard must read `writable`.

### Code signing, or why the grant may not stick

The grant is tied to the binary's code signature. Go's linker leaves an ad-hoc
*linker-signed* signature whose identifier is `a.out`; TCC cannot hold a grant
against that, so the daemon could be added to Input Monitoring and every device
open still failed. `make build` therefore re-signs with a stable identifier.

Ad-hoc signing changes the binary's hash on every rebuild, so the entry must be
removed and added again each time. To keep the grant, sign with a self-signed
certificate. In **Keychain Access** → menu *Keychain Access* → *Certificate
Assistant* → *Create a Certificate*:

- Name: `zmk-vim-mode-dev`
- Identity Type: *Self Signed Root*
- Certificate Type: *Code Signing* — the dialog defaults to *SSL Client*

Then build with it and grant Input Monitoring once:

```bash
make install CODESIGN_IDENTITY=zmk-vim-mode-dev
```

`security find-identity -v -p codesigning` lists what the keychain has.

### What differs from Linux

- **LED writes** go through IOKit `IOHIDDeviceSetValueMultiple`: all five
  indicators in one call, so one output report carries the whole code and the
  firmware never decodes a half-written one. (`IOHIDDeviceSetReport` is kept as
  a fallback; on its own it was rejected by the system's keyboard driver.)
  Num and Caps Lock keep whatever the host has lit.
- **No clobber to repair.** macOS never writes Compose, Kana or Scroll Lock, so
  a code is re-asserted only when a device appears and after wake
  (`IORegisterForSystemPower`). There is no `EV_LED`-style echo to watch.
- **Frontmost app** comes from `NSWorkspace` (bundle identifiers:
  `com.mitchellh.ghostty`, `com.microsoft.VSCode`, `md.obsidian`), polled ten
  times a second — its change notifications are delivered only through a Cocoa
  main run loop, which a Go daemon does not run.
- **Window titles need the Accessibility permission.** They are read through
  the Accessibility API (`AXUIElement`), not Screen Recording. Grant it and
  VSCode's `[${focusedView}]` marker works exactly as on Linux, tool windows
  included; without it the daemon sees no titles and only the companion
  extension and the embedded Neovim report VSCode's state. `zmk-vim-mode
  doctor` opens the system dialog; the entry is System Settings → Privacy &
  Security → Accessibility → `~/.local/bin/zmk-vim-mode`.
- **No accessibility bus**: AT-SPI2 is Linux-only, so `--atspi` does nothing
  here and a quick input opened with the mouse is not detected.

`zmk-vim-mode install --vscode --obsidian`, `status` and `doctor` work the same.

### Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `devices`: no keyboards | the keyboard serves another host | `zmk-vim-mode hid-scan` lists what this Mac sees; a ZMK keyboard talks to one BLE profile at a time |
| `hid-scan` shows it, `devices` does not | daemon still on the old binary | `make install` (it restarts the agent) |
| `NOT writable`, *not permitted* | Input Monitoring missing for **this** build | remove and re-add `~/.local/bin/zmk-vim-mode`, then `launchctl kickstart -k gui/$UID/dev.rafaelromao.zmk-vim-mode` |
| writes succeed, layers do not move | the keyboard is acting on another endpoint | ZMK keeps indicators per endpoint and only the selected one raises the event: check the keyboard's output (USB vs BLE) |
| `bootstrap`: `5: Input/output error` | the agent is already loaded | `launchctl kickstart -k gui/$UID/dev.rafaelromao.zmk-vim-mode` |
| VSCode's terminal or sidebar keeps the vim layers | no Accessibility permission, so no window titles | `zmk-vim-mode doctor` (it asks), then restart the agent |
| nothing works, unclear why | — | run it in the foreground from a terminal that already has Input Monitoring: `launchctl bootout gui/$UID/dev.rafaelromao.zmk-vim-mode; zmk-vim-mode daemon --log-level debug` |

## Commands

```
zmk-vim-mode daemon [--atspi]   run the daemon (normally via the user service)
zmk-vim-mode status             current decision, frontmost app, widget focus, clients, devices
zmk-vim-mode devices            keyboards the daemon can write to, and the last code sent to each
zmk-vim-mode set <mode>         manual override; repeating the same mode returns to auto
zmk-vim-mode doctor             daemon, devices, permissions, old watchers, and the editor setups
zmk-vim-mode install [flags]    service, Neovim spec, --vscode, --obsidian, --atspi, --udev, --tmux
zmk-vim-mode uninstall          remove the service (config is left alone)
zmk-vim-mode atspi-watch        Linux: accessibility-bus focus events with the classifier's verdict
zmk-vim-mode hid-scan [--all]   macOS: HID keyboards this host sees and the LEDs they expose
zmk-vim-mode version
```

`set` is the escape hatch for anything the daemon cannot detect (an SSH
session, a screen-sharing app). Bind it to a key with
`contrib/hyprland-bind.conf`. `atspi-watch` and `hid-scan` are the two
diagnostics: the first says how a widget was classified, the second whether the
keyboard is on this host at all.

## Development

```bash
make test          # Go, Neovim and firmware-policy suites
make lint          # gofmt + go vet, for this platform and for Linux
make cross         # static binaries for the Omarchy box (linux/amd64, linux/arm64)
```

Linux builds are pure Go and static; macOS needs cgo for IOKit and Cocoa, and
signs the result (see *Code signing* above). `make cross` stays CGO-free, so it
can be run from either machine.

`scripts/spike-linux.sh` covers the bring-up checks on Linux: find the
keyboard's device nodes, confirm the firmware exposes the three LEDs, write a
code by hand, and watch the `EV_LED` echoes that reveal a clobber.

The firmware's decode and timing rules live in `firmware/src/code_policy.h`,
which depends on nothing but `stdint`, so they are unit-tested on the host
(`make test-firmware`) rather than only on hardware.

## Licence

MIT. See [LICENSE](LICENSE).
