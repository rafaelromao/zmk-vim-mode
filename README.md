# zmk-vim-mode

Keeps a ZMK keyboard's layers in sync with the editor's real vim state, so the
keyboard is in NORMAL when the editor is in normal mode, in INSERT when you are
typing, and out of vim layers entirely when keys must pass through untouched.

![The keyboard following the editor: typing on the base layer, then NORMAL with h j k l under four
fingers of the right hand, v into VISUAL to select a word, yank and put it back, i into INSERT and
Esc back again](docs/img/vim-layers.gif)

*The author's [Diamond](https://github.com/rafaelromao/keyboards), drawn by
[zmk-layer-hud](https://github.com/rafaelromao/zmk-layer-hud) from the keyboard's own layer
reports — [how it is made](docs/hud/README.md).*

Everything is in this repository:

| Part | Path | Role |
|---|---|---|
| ZMK module | `firmware/`, `zephyr/` | decodes the mode on the keyboard and switches layers |
| Host daemon | `cmd/`, `internal/` | decides the mode from the focused app and the editors, writes it to the keyboard |
| Neovim plugin | `lua/`, `plugin/` | reports Neovim's effective mode over a unix socket |
| Editor plugins | `editors/` | the VSCode companion and the Obsidian and IntelliJ plugins, speaking the same protocol |
| Status bar indicators | `bars/` | show the mode in the macOS menu bar or the Omarchy bar |

## Why

`hjkl` sits on QWERTY's home row by accident of history. Move to a modern
alternative layout and those four letters scatter. On Gallium, for instance,
`j` and `k` share the right index finger's inner column and `l` moves to the
left hand:

```
b l d c v   j y o u ,
n r t s g   p h a e i
x q m w z   k f ' ; .
```

Colemak, Dvorak and Graphite each scatter them differently; none of them keeps
the row. Remapping vim to match means fighting every plugin, tutorial and
machine you ssh into; learning the scattered positions makes your layout worse
at the thing you do most. A vim layer removes the compromise. While the editor
is in normal mode the keyboard is issuing commands, not typing letters, so that
layer can be a map chosen for how vim is actually used:

- **motions on the home row**, `h j k l` under your strongest fingers, whatever
  your base layout does with those letters;
- **operators under the other hand**, so `d`, `y`, `c`, `v` are one comfortable
  key each;
- **two-key commands as one key**: `dd`, `yy`, `gg` as macros, `^D`/`^U` with
  no modifier;
- **prime positions for `:` and `/`**, far more frequent than `;` or `'` in
  normal mode.

None of it costs you anything while typing: the layer is gone the moment the
editor goes back to insert. That is the trade this project exists to make, a
dedicated command layout that appears exactly when the editor is expecting
commands and disappears exactly when it is not.

And vim itself needs no configuration. The keyboard sends real `h`, `j`, `k`,
`l` keycodes; the layer only decides which physical key produces them. There is
no `noremap` to maintain, nothing that breaks when a plugin expects `dw` to
work, and nothing to install on the servers you ssh into. Stock vim, stock
plugins, a keyboard that speaks their language.

The hard part is knowing which mode the editor is in. A keyboard that guesses
from keystrokes goes wrong as soon as a plugin, a mouse click or `:startinsert`
changes the mode behind its back. zmk-vim-mode asks the editor instead, and
tells the keyboard.

## Requirements

- **A ZMK keyboard whose firmware you build**, from a zmk-config of your own,
  locally or with GitHub Actions. USB and Bluetooth both work. On a split
  keyboard only the central half, or the dongle, needs the module.
- **macOS or Linux** on the computer. On Linux, following the focused window
  needs [Hyprland](https://hyprland.org) (Omarchy ships it); under another
  compositor the daemon still runs, but only editors with a plugin report their
  mode.
- **Go 1.26 or newer and `make`** to build the daemon. On macOS also the Xcode
  Command Line Tools (`xcode-select --install`), for cgo.
- **An editor.** Neovim, VSCode with
  [vscode-neovim](https://github.com/vscode-neovim/vscode-neovim), Obsidian with
  vim key bindings, and IntelliJ with IdeaVim report their exact mode. Any other
  vim-like app works too, with the keyboard following the mode itself (see
  [Modes](#modes)).
- **Optionally**, [Hammerspoon](https://www.hammerspoon.org) on macOS or
  Omarchy 4 on Linux, for the [status bar indicator](#status-bar-indicator).

## Quick start

1. **Keyboard.** Add the module to your zmk-config and the `vim_sync` node and
   four vim layers to your keymap, as in [Keyboard setup](#keyboard-setup), then
   build and flash (on a split, the central half or the dongle).
2. **Computer.** Clone and install:

   ```bash
   git clone https://github.com/rafaelromao/zmk-vim-mode
   cd zmk-vim-mode
   make install
   ```

   That builds the daemon, runs it as a user service, and sets up the editors
   and the status bar it finds. [Host setup](#host-setup) has the details.
3. **macOS only:** add the daemon under Input Monitoring and Accessibility. Both
   panes open at the end of `make install` ([why](#macos-permissions)).
4. **Check** the setup, then drive the keyboard by hand:

   ```bash
   zmk-vim-mode doctor
   zmk-vim-mode set normal   # the keyboard switches to its NORMAL layer
   zmk-vim-mode set auto     # back to following the editor
   ```

Then open Neovim and press `i`, `Esc` and `v`: the keyboard's layers follow.

## Modes

The daemon picks one mode at a time, from the focused app and what its editor
reports, and the keyboard switches layers to match:

| Mode | When | The keyboard |
|---|---|---|
| **normal** | the editor is in normal mode | `VIM_NORMAL` |
| **insert** | typing (Neovim's Replace counts as insert) | `VIM_INSERT`, usually transparent: your base layout |
| **visual** | selecting | `VIM_VISUAL` on top of `VIM_NORMAL` |
| **cmdline** | typing a `:` command or a search | `VIM_CMDLINE` |
| **raw** | the letters are commands of something else: a file explorer, a picker, a terminal job, a pending `<leader>` | no vim layers: keys pass through untouched |
| **legacy** | a vim-like editor that reports nothing: vim over SSH, Helix, an IDE without the plugin | `VIM_NORMAL`, and the keyboard follows the mode itself |
| **off** | any other app | no vim layers |

`zmk-vim-mode status` shows the current mode and why, and
`zmk-vim-mode set <mode>` overrides it by hand.

## Keyboard setup

### Add the module

zmk-vim-mode is a Zephyr module. Add it to `config/west.yml` in your
zmk-config, next to ZMK itself:

```yaml
manifest:
  remotes:
    - name: zmkfirmware
      url-base: https://github.com/zmkfirmware
    - name: rafaelromao
      url-base: https://github.com/rafaelromao
  projects:
    - name: zmk
      remote: zmkfirmware
      revision: main
      import: app/west.yml
    - name: zmk-vim-mode
      remote: rafaelromao
      revision: main
  self:
    path: config
```

Nothing else in the build changes. The module switches itself on, along with
`CONFIG_ZMK_HID_INDICATORS`, the HID LED support it relies on, as soon as the
keymap has the `vim_sync` node below. It builds only on the central half of a
split, where the host's LED reports arrive, so peripherals need no reflash.
[docs/keyboards-repo.md](docs/keyboards-repo.md) walks through a real config
that uses it.

### The least you need

Four layers and one node. The layers may be entirely `&trans` to begin with —
a vim layer is useful because you *put* something on it, but nothing breaks
while it is empty, and `&to` leaves the base layer showing through.

```c
#include <dt-bindings/zmk/hid_usage.h>

#define BASE        0
#define VIM_NORMAL  1
#define VIM_VISUAL  2
#define VIM_INSERT  3
#define VIM_CMDLINE 4

/ {
    vim_sync {
        compatible = "zmk,hid-indicator-code-listener";
        indicators = <HID_USAGE_LED_COMPOSE HID_USAGE_LED_KANA HID_USAGE_LED_SCROLL_LOCK>;
        managed-layers = <VIM_NORMAL VIM_VISUAL VIM_INSERT VIM_CMDLINE>;

        normal        { code = <1>; layers = <VIM_NORMAL>; };
        insert        { code = <2>; layers = <VIM_INSERT>; };
        visual        { code = <3>; layers = <VIM_NORMAL VIM_VISUAL>; };
        legacy        { code = <4>; layers = <VIM_NORMAL>; bindings = <&kp ESC>; };
        cmdline       { code = <5>; layers = <VIM_CMDLINE>; };
        raw           { code = <6>; };
        legacy_silent { code = <7>; layers = <VIM_NORMAL>; };
        // code 0 is implicit: every managed layer off.
    };
};
```

Each entry maps one of the [modes](#modes) to the layers it turns on; the
numbers are the codes the host sends for them ([how](#the-codes-on-the-wire)).

With an editor that reports its mode — Neovim, or VSCode through
vscode-neovim, or Obsidian — that is the whole keymap side. The host names the
mode and the module switches layers; the keyboard never has to guess.

### Layer order

**Keep the vim layers below your other layers.** Layer priority in ZMK is
numeric, so with `NAV` and `SYM` above them, holding a nav key still works
while vim layers are active. Put them above and the vim layer would shadow
everything you hold.

### Inferring modes on the keyboard

Codes 4 and 7, the two `legacy` entries in that node, say only *"a vim-like
editor has focus"*: the mode is unknown, because the editor has no plugin (vim
over SSH, a JetBrains IDE without the plugin below, Helix, anything with vim
keys of its own), or because you entered vim mode by hand. Then the keyboard
has to follow the mode itself, by watching the keys that change it.

These are the transitions worth implementing. `^C` behaves as `Esc`
throughout; `I A O S C` are the shifted forms of the letters beside them.

| In layer | Key | Goes to | Why |
|---|---|---|---|
| NORMAL | `i` `a` `o` `s` `I` `A` `O` `S` `C` | INSERT | the insert commands |
| NORMAL | `c` | INSERT | see the caveat below |
| NORMAL | `R` | INSERT | replace types like insert |
| NORMAL | `v` `V` `^V` | VISUAL | the three visual modes |
| NORMAL | `:` `/` `?` | CMDLINE | command line and search |
| INSERT | `Esc` | NORMAL | |
| VISUAL | `Esc` | NORMAL | |
| VISUAL | `v` | NORMAL | pressing `v` again leaves visual |
| VISUAL | `c` `s` | INSERT | change the selection |
| VISUAL | `d` `x` `y` `p` `J` `=` `~` `u` | NORMAL | operators that consume the selection |
| VISUAL | `:` | CMDLINE | `:'<,'>` |
| CMDLINE | `Enter` `Esc` | NORMAL | the line is submitted or abandoned |

Nothing tracks leaving vim altogether (`ZZ`, `:q`, closing the editor) — that
is the host's job, and it sends code 0.

### A sample implementation

One macro helper covers nearly every row of that table: tap the key, then
switch layer. ZMK's `&to` is the whole trick — it activates one layer and drops
every other except the base, which is exactly how vim modes behave. Visual is
the one exception, and uses `&tog` so it stacks on normal instead of replacing
it.

```c
// Tap KEY, then make LAYER the only active layer.
#define VIM_KEY(NAME, KEY, LAYER) \
    NAME: NAME { \
        compatible = "zmk,behavior-macro"; \
        #binding-cells = <0>; \
        bindings = <&kp KEY &to LAYER>; \
    };

/ {
    macros {
        VIM_KEY(vim_i,     I,     VIM_INSERT)
        VIM_KEY(vim_a,     A,     VIM_INSERT)
        VIM_KEY(vim_o,     O,     VIM_INSERT)
        VIM_KEY(vim_s,     S,     VIM_INSERT)
        VIM_KEY(vim_c,     C,     VIM_INSERT)
        VIM_KEY(vim_r,     R,     VIM_INSERT)
        VIM_KEY(vim_colon, COLON, VIM_CMDLINE)
        VIM_KEY(vim_slash, SLASH, VIM_CMDLINE)
        VIM_KEY(vim_esc,   ESC,   VIM_NORMAL)
        VIM_KEY(vim_enter, RET,   VIM_NORMAL)
        VIM_KEY(vim_d,     D,     VIM_NORMAL)   // visual-mode operators
        VIM_KEY(vim_y,     Y,     VIM_NORMAL)
        VIM_KEY(vim_p,     P,     VIM_NORMAL)
        VIM_KEY(vim_x,     X,     VIM_NORMAL)

        // Visual sits *on top of* normal, the way code 3 does, so it toggles
        // rather than replaces. The same macro serves both rows of the table:
        // `v` in normal turns VISUAL on, `v` in visual turns it off again.
        vim_v: vim_v {
            compatible = "zmk,behavior-macro";
            #binding-cells = <0>;
            bindings = <&kp V &tog VIM_VISUAL>;
        };
    };
};
```

Then place them. Only the keys that change mode need an entry; everything else
is `&trans`, so your base layout shows through — or your own vim bindings, if
the point of the NORMAL layer is to move `hjkl` somewhere better.

```c
vim_normal_layer {
    display-name = "NORMAL";
    bindings = <
        // ... &trans for the keys that do not change mode ...
        &vim_i      &vim_a     &vim_o     &vim_s    &vim_c
        &vim_r      &vim_v     &vim_colon &vim_slash
    >;
};

vim_insert_layer {
    display-name = "INSERT";
    bindings = <
        // all &trans except:
        &vim_esc
    >;
};

vim_visual_layer {
    display-name = "VISUAL";
    bindings = <
        // sits over VIM_NORMAL, so it only needs the keys that leave visual;
        // motions fall through to the layer underneath:
        &vim_esc    &vim_v     &vim_c     &vim_d    &vim_y    &vim_p   &vim_x
    >;
};

vim_cmdline_layer {
    display-name = "CMDLINE";
    bindings = <
        // all &trans except:
        &vim_esc    &vim_enter
    >;
};
```

Shifted variants (`I`, `A`, `O`, `C`, `V`) are the same macros behind a
mod-morph, or simply the same key: `&vim_i` with Shift held already sends `I`
and lands in INSERT, which is the correct outcome.

Two things this simple version gets wrong, both survivable:

- **`c{motion}`** — `cw` switches to INSERT on the `c`, so the `w` is typed
  with the NORMAL layer already gone. Harmless when NORMAL leaves letters
  alone; when NORMAL remaps letters, the motion key is the wrong one. The fix
  is a short-lived operator-pending layer.
- **counts and registers** — `3x`, `"ayy`: the digits and the register name
  pass through NORMAL unchanged, which is right, but `x` in `3x` still returns
  to NORMAL, which it already was. No harm.

Anything the keyboard gets wrong here is corrected by the host within a
keystroke as soon as a reporting editor is focused: local inference only has
to be good enough for the editors that cannot speak.

A complete, compilable 34-key keymap built this way, with a Gallium base,
home-row mods and the four vim layers, is in
[docs/example-keymap.md](docs/example-keymap.md).

## Host setup

### Install

```bash
git clone https://github.com/rafaelromao/zmk-vim-mode
cd zmk-vim-mode
make install
```

One command, both platforms. It:

- builds the binary, installs it in `~/.local/bin`, and adds that directory to
  your shell profile when it is not already on `PATH`;
- writes the user service (a launchd agent on macOS, a systemd user unit on
  Linux) and starts it;
- sets up the editors it finds, skipping any that is not installed: the Neovim
  plugin spec (an existing spec is never touched), VSCode, Obsidian, and
  IntelliJ, whose plugin it builds against your IDE — the first build downloads
  Gradle, so it takes a while;
- puts the [status bar indicator](#status-bar-indicator) in your bar: the
  Omarchy widget on Linux, the Hammerspoon menu bar item on macOS, each
  skipped when its host is missing;
- on Linux, installs the udev rule (the one `sudo` prompt) and enables the
  accessibility bus;
- on macOS, creates the code-signing certificate if you have none (your login
  password), signs the binary, and opens the two Privacy & Security panes.

The `PATH` line goes in `~/.zshrc`, `~/.bashrc` (`~/.bash_profile` on macOS)
or `config.fish` depending on `$SHELL`, is marked so a second install never
stacks a duplicate, and only takes effect in a **new** shell — no process can
change the `PATH` of the shell that started it. Opt out with `--no-path`, and
out of the panes with `--no-open`.

Four things it cannot do for you:

- **flash the firmware module**;
- **restart the editors**, so they load their new plugins;
- **`set -g focus-events on`** in `~/.tmux.conf`, or Neovim never sees
  `FocusLost` inside tmux;
- on macOS, **grant the two permissions** below — it opens the panes, but only
  System Settings itself may write TCC.

Then check everything:

```bash
zmk-vim-mode doctor
```

### macOS permissions

Two grants, each added by hand under System Settings → Privacy & Security,
both pointing at `~/.local/bin/zmk-vim-mode` (`+`, then ⌘⇧G to type the path).
`make install` opens both panes for you at the end of the run — it cannot fill
them in, because TCC's database is SIP-protected and System Settings is the
only thing allowed to write it:

- **Input Monitoring** — required to open the keyboard's HID device, so
  without it nothing reaches the keyboard at all. `zmk-vim-mode devices` shows
  `writable` once it is in place.
- **Accessibility** — lets the daemon read window titles, which is how tool
  windows are detected (VSCode's terminal, sidebar, panels). Without it only
  the editors' own plugins report.

A background agent is never prompted for either, which is why they have to be
added manually. `zmk-vim-mode doctor` reports what the **daemon** was granted —
the only answer that counts, since macOS judges such a request by the
responsible process, and a CLI run from a terminal is judged on that
terminal's permissions.

Both grants are tied to the binary's code signature, and Go stamps the version
into every build, so each rebuild would void them. `make install` avoids that
by signing with a self-signed certificate, which it creates on the first run
that finds no signing identity — it asks to allow `codesign` to use the key,
then for your login password. To create it separately:

```bash
make codesign-cert
```

Without a certificate the binary is signed ad-hoc and both permissions must be
removed and re-added after every rebuild; that is what happens in CI, or in any
build with no terminal to ask on. `make build` says which of the two it used.

### Editors

| Editor | Mode source | Tool-window focus | Setup |
|---|---|---|---|
| Neovim in a terminal, Neovide | the Neovim plugin | the plugin: `raw` for pickers, the terminal, a pending `<leader>` | `zmk-vim-mode install --nvim` (also done by `make install`) |
| VSCode | the same plugin, inside [vscode-neovim](https://github.com/vscode-neovim/vscode-neovim) | the window title, which `install --vscode` makes carry `[${focusedView}]` (read from Hyprland on Linux, the Accessibility API on macOS); a companion extension for quick inputs and non-text editors; the accessibility bus for anything opened with the mouse (Linux) | `zmk-vim-mode install --vscode` (also done by `make install`, with `--atspi` on Linux), then [editors/vscode](editors/vscode/README.md) |
| Obsidian | own plugin (CodeMirror vim events) | own plugin (`focusin`) | `zmk-vim-mode install --obsidian` (also done by `make install`), then [editors/obsidian](editors/obsidian/README.md) |
| IntelliJ | own plugin (IdeaVim's mode listener) | own plugin: editor focus, and `EditorKind` to keep the terminal and consoles out — they are editors too | `zmk-vim-mode install --intellij` (also done by `make install`) — needs IdeaVim; it builds the plugin against your IDE, see [editors/intellij](editors/intellij/README.md) |
| anything else | none: `legacy`, the keyboard infers | — | nothing; `set raw` when a tool window traps you |

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

### Status bar indicator

The mode the keyboard is in, where you can see it: NORMAL, INSERT, VISUAL,
CMDLINE, VIM (legacy) or RAW behind a Neovim glyph, `VIM ?` while the daemon is
down, and nothing at all while vim mode is off. It is the decision the keyboard
just acted on, not a second guess at it.

| Bar | What gets installed | Setup |
|---|---|---|
| macOS menu bar | a [Hammerspoon](https://www.hammerspoon.org) Spoon, `~/.hammerspoon/Spoons/ZmkVimMode.spoon`, and the lines in `init.lua` that start it | `zmk-vim-mode install --hammerspoon` (also done by `make install`), see [bars/hammerspoon](bars/hammerspoon/README.md) |
| Omarchy Quattro bar | a bar widget plugin, `~/.config/omarchy/plugins/rafaelromao.zmk-vim-mode`, listed in `bar.layout.right` of `shell.json` | `zmk-vim-mode install --omarchy` (also done by `make install`), see [bars/omarchy](bars/omarchy/README.md) |
| any other bar | nothing: `zmk-vim-mode status --bar` prints the same answer as one Waybar-style JSON line (`text`, `tooltip`, `class`) | a module that runs it every second, below |

Both widgets only draw what `status --bar` prints, so they always agree, and
it answers even while the daemon is down. The installers write the binary's
full path into the widget, since a bar does not see your shell's `PATH`, and
they only ever add to `init.lua` and `shell.json` (keeping a backup of
`shell.json`); either file may also be a symlink, which stays one.

A Waybar module, for instance:

```jsonc
"custom/vim-mode": {
  "exec": "zmk-vim-mode status --bar",
  "return-type": "json",
  "interval": 1,
  "format": "\ue6ae {}"
}
```

### Following focus through the accessibility bus (Linux)

Two things nothing above can see: a quick input opened with the mouse, and the
exact moment focus returns to the editor. Both are visible on AT-SPI2, the
Linux accessibility bus, which every toolkit reports focus changes to — when
accessibility is on. `make install` turns it on; `zmk-vim-mode install --atspi`
does it alone, and dropping `--atspi` from `INSTALL_FLAGS` in the Makefile
leaves it off.

It also adds `--force-renderer-accessibility` to `~/.config/code-flags.conf`
(read by Arch's `code` wrapper): Electron builds the accessibility tree of its
web content only with that switch — the bus flags alone reach GTK and Qt, not
VSCode's DOM. Restart VSCode afterwards.

The daemon then keeps one connection to the bus, registers as a focus
listener and classifies each focused widget of the frontmost VSCode: the
Monaco code editor → the clients decide; anything else (the palette's input,
the terminal, a tree, a rename box, the find widget) → raw. It also tells the
VSCode companion and the embedded Neovim when the editor regained focus, so
their own guesses clear instantly.

What it costs, and why it is opt-in: the daemon sets both `org.a11y.Status`
flags (`IsEnabled`, `ScreenReaderEnabled`) on the session, which is what a
screen reader does — GTK, Qt and Chromium applications start maintaining
accessibility trees (a little CPU and memory, nothing visible).
`install --vscode` already sets `editor.accessibilitySupport: off` so VSCode
does not switch Monaco into screen-reader mode because of it. Applications read
the flag at startup: restart VSCode after enabling. The daemon reads only
roles, labels and HTML tag/class names of focused widgets — never text — and
logs labels at debug only.

`zmk-vim-mode atspi-watch` prints every focus event with the classifier's
verdict; if a VSCode update renames a widget, that is where the new name shows
up. `zmk-vim-mode status` shows `focus : elsewhere (input.input …)` while a
quick input is open. No D-Bus library is involved: `internal/dbus` is a
300-line client for the handful of calls this needs.

### Upgrading and uninstalling

To upgrade, pull and install again. `make install` restarts the service, so it
never leaves the old binary running:

```bash
git pull && make install
```

Rebuild the firmware too when the module has changed.

`make uninstall` stops and removes the service and the binary. It leaves your
configuration alone, so these stay until you remove them:

- the editor integrations: the Neovim spec, the VSCode extension and settings,
  the Obsidian and IntelliJ plugins;
- the status bar indicator: `~/.hammerspoon/Spoons/ZmkVimMode.spoon` and its
  lines at the end of `init.lua`, or the
  `~/.config/omarchy/plugins/rafaelromao.zmk-vim-mode` plugin and its entry in
  `shell.json`. Neither shows anything once the binary is gone;
- the `PATH` line in your shell profile, marked `# added by zmk-vim-mode`;
- on Linux, the udev rule: `sudo rm /etc/udev/rules.d/60-zmk-vim-mode.rules`.

## Commands

```
zmk-vim-mode daemon [--atspi]   run the daemon (normally via the user service)
zmk-vim-mode status             current decision, frontmost app, widget focus, clients, devices
                                --json: the daemon's full state; --bar: one line for a status bar
zmk-vim-mode devices            keyboards the daemon can write to, and the last code sent to each
zmk-vim-mode set <mode>         manual override: off, normal, insert, visual, cmdline, raw, legacy or auto;
                                --ttl 30s lets it lapse, --sticky keeps it when another app takes focus,
                                repeating the same mode returns to auto
zmk-vim-mode doctor             daemon, devices, permissions, the editor setups and the status bar indicator
zmk-vim-mode install [flags]    service, PATH entry, Neovim spec, --vscode, --obsidian, --intellij, --atspi, --udev, --tmux,
                                --hammerspoon (macOS menu bar), --omarchy (Omarchy bar widget)
                                --no-path keeps your shell profile untouched; --no-open leaves the macOS panes closed
zmk-vim-mode uninstall          remove the service (config is left alone)
zmk-vim-mode atspi-watch        Linux: accessibility-bus focus events with the classifier's verdict
zmk-vim-mode hid-scan [--all]   macOS: HID keyboards this host sees and the LEDs they expose
zmk-vim-mode version
```

`set` is the escape hatch for anything the daemon cannot detect (an SSH
session, a screen-sharing app). On Hyprland, `contrib/hyprland-bind.conf` binds
it to keys. `atspi-watch` and `hid-scan` are the two
diagnostics: the first says how a widget was classified, the second whether the
keyboard is on this host at all.

## Troubleshooting

Start with `zmk-vim-mode doctor`. It checks the daemon, the keyboards, the
permissions, the editors and the status bar, and prints a fix for each problem
it finds.

### macOS

| Symptom | Cause | Fix |
|---|---|---|
| `zmk-vim-mode: command not found` | `~/.local/bin` is not on `PATH` | `make install` appends the line to your shell profile, but only a **new** shell reads it: `exec $SHELL`. A profile that writes `"~/bin"` inside double quotes leaves an unexpanded tilde, which names nothing — use `$HOME` |
| grants lost again after a rebuild | the binary was signed ad-hoc | no signing identity existed at build time (or the build had no terminal to ask on): `make codesign-cert`, then `make install` |
| `devices`: no keyboards | the keyboard serves another host | `zmk-vim-mode hid-scan` lists what this Mac sees; a ZMK keyboard talks to one BLE profile at a time |
| `hid-scan` shows it, `devices` does not | daemon still on the old binary | `make install` (it restarts the agent) |
| `NOT writable`, *not permitted* | Input Monitoring missing for **this** build | remove and re-add `~/.local/bin/zmk-vim-mode`, then `launchctl kickstart -k gui/$UID/dev.rafaelromao.zmk-vim-mode` |
| writes succeed, layers do not move | the keyboard is acting on another endpoint | ZMK keeps indicators per endpoint and only the selected one raises the event: check the keyboard's output (USB vs BLE) |
| `bootstrap`: `5: Input/output error` | the agent is already loaded | `launchctl kickstart -k gui/$UID/dev.rafaelromao.zmk-vim-mode` |
| VSCode's terminal or sidebar keeps the vim layers | no Accessibility permission, so no window titles | add `~/.local/bin/zmk-vim-mode` under Accessibility, then `launchctl kickstart -k gui/$UID/dev.rafaelromao.zmk-vim-mode`; `doctor` reports what the **daemon** was granted, which is the only answer that counts |
| Obsidian reports nothing | the plugin is in the vault but not enabled | Obsidian rewrites its plugin list on exit, so `install --obsidian` cannot enable it while Obsidian runs: quit Obsidian and run it again, or enable *ZMK Vim Mode* in Settings → Community plugins |
| nothing works, unclear why | — | run it in the foreground from a terminal that already has Input Monitoring: `launchctl bootout gui/$UID/dev.rafaelromao.zmk-vim-mode; zmk-vim-mode daemon --log-level debug` |

### Linux

| Symptom | Cause | Fix |
|---|---|---|
| `zmk-vim-mode: command not found` | `~/.local/bin` is not on `PATH` | `make install` appends it to `~/.bashrc` (or your shell's profile), but only a **new** shell reads it: `exec $SHELL` |
| daemon not reachable | the user service is not running | `systemctl --user enable --now zmk-vim-mode.service`; its log: `journalctl --user -u zmk-vim-mode -n 40` |
| `devices`: no keyboards | firmware without the module, or the keyboard is on another host | flash the module (it turns HID indicators on); check the keyboard is connected here, since a ZMK keyboard talks to one BLE profile at a time, and that the udev rule is installed |
| `devices`: `NOT writable` | the udev rule is missing, or the keyboard connected before it was installed | `make install` installs it (the one `sudo` prompt); reconnect USB, or re-pair a Bluetooth keyboard, so the rule applies to its new device nodes |
| writable at the desktop, not over SSH | the rule's `uaccess` needs an active local session | use the group form documented in `contrib/udev/60-zmk-vim-mode.rules` |
| editors switch layers, other vim-like apps never do | no Hyprland, so no focus backend: the daemon only hears editor plugins | window titles come from Hyprland; under other compositors only the editors' own plugins report |
| writes succeed, layers do not move | the keyboard is acting on another endpoint | ZMK keeps indicators per endpoint and only the selected one raises the event: check the keyboard's output (USB vs BLE) |

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

Three of those five are free, assuming the operating system never *sets* **Compose**,
**Kana** or **Scroll Lock**. Num Lock and Caps Lock are deliberately
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

### What differs on macOS

- **LED writes** go through IOKit `IOHIDDeviceSetValueMultiple`: all five
  indicators in one call, so one output report carries the whole code and the
  firmware never decodes a half-written one. (`IOHIDDeviceSetReport` is kept as
  a fallback; on its own it was rejected by the system's keyboard driver.)
  Num and Caps Lock keep whatever the host has lit.
- **No clobber to repair.** macOS never writes Compose, Kana or Scroll Lock, so
  a code is re-asserted only when a device appears and after wake
  (`IORegisterForSystemPower`). There is no `EV_LED`-style echo to watch.
- **Frontmost app** is asked of the accessibility system
  (`kAXFocusedApplication`), or of the window list (`CGWindowList`) when that
  permission is missing, and polled ten times a second. Apps are matched by
  bundle identifier: `com.mitchellh.ghostty`, `com.microsoft.VSCode`,
  `md.obsidian`. `NSWorkspace.frontmostApplication` is only a last resort: it
  updates through Cocoa notifications delivered to a main run loop, which a
  daemon does not run, so it answers with whatever was frontmost at startup
  for as long as the process lives.
- **Window titles need the Accessibility permission.** They are read through
  the Accessibility API (`AXUIElement`), not Screen Recording. Grant it and
  VSCode's `[${focusedView}]` marker works exactly as on Linux, tool windows
  included; without it the daemon sees no titles and only the companion
  extension and the embedded Neovim report VSCode's state. `make install`
  opens the pane and `zmk-vim-mode doctor` opens the system dialog; the entry
  to add is System Settings → Privacy & Security → Accessibility →
  `~/.local/bin/zmk-vim-mode`.
- **No accessibility bus**: AT-SPI2 is Linux-only, so `--atspi` does nothing
  here and a quick input opened with the mouse is not detected.

Everything else is the same: `make install`, `status`, `doctor`.

Why the signature matters, since it is the one thing with no visible cause:
Go's linker leaves an ad-hoc *linker-signed* signature whose identifier is
`a.out`, and TCC cannot hold a grant against that — the daemon can sit in the
permission list with every device open still refused. `make build` re-signs
with a stable identifier, and `make install` makes that identifier a
certificate — it creates `zmk-vim-mode-dev` on the first run that finds no
signing identity, so the grants outlive later rebuilds. `make codesign-cert`
does only that step. By hand the certificate is
**Keychain Access** → menu *Keychain Access* → *Certificate Assistant* →
*Create a Certificate*: Name `zmk-vim-mode-dev`, Identity Type *Self Signed
Root*, Certificate Type *Code Signing* (the dialog opens on *SSL Client*).
`security find-identity -v -p codesigning` lists what the keychain has.

## Development

```bash
make test          # Go, Neovim and firmware-policy suites
make lint          # gofmt + go vet, for this platform and for Linux
make cross         # static Linux binaries (linux/amd64, linux/arm64)
```

Linux builds are pure Go and static; macOS needs cgo for IOKit and Cocoa, and
signs the result (see *What differs on macOS*). `make cross` stays CGO-free, so
it runs on macOS and Linux alike.

`scripts/spike-linux.sh` covers the bring-up checks on Linux: find the
keyboard's device nodes, confirm the firmware exposes the three LEDs, write a
code by hand, and watch the `EV_LED` echoes that reveal a clobber.

The firmware's decode and timing rules live in `firmware/src/code_policy.h`,
which depends on nothing but `stdint`, so they are unit-tested on the host
(`make test-firmware`) rather than only on hardware.

## Licence

MIT. See [LICENSE](LICENSE).
