# Showcase video kit

Everything needed to record *Rafael Romão's keymap and supporting tools*: the script, an
on-screen layer HUD, committed demo content with no private data, a one-command editor reset,
and an automated rehearsal. `HANDOFF.md` is the long-form state of the work, including what is
different on the Omarchy box.

| Path | What |
|---|---|
| `SCRIPT.md` | the video script: beats, timings, narration, expected `status` per take, shot list |
| `HANDOFF.md` | state, decisions, lessons, and the Linux port notes for the next agent |
| `keymap/build.py` | reads the keymap-drawer YAML in `~/projects/keyboards` → `hud/keymap.json` + `hud/keymap.js` |
| `hud/` | the HUD pages (`index.html` + `hud.js` + `hud.css`, `keys.html`) and the macOS host (`hud.lua`, `place.lua`, `rehearse.lua`, `start.sh`) |
| `linux/` | the Omarchy host: `keyfeed.py`, `hud.sh`, `prepare.sh`, `rehearse.py` (untested at handoff) |
| `Demo/` | the Obsidian vault used on camera (open it as a vault; its runtime state is ignored) |
| `demo-go/`, `demo-go.code-workspace` | Go module for Neovim and VS Code |
| `demo-java/` | plain Java project for IntelliJ IDEA |
| `env/` | `ghostty-demo.conf` and `.zshrc` (demo shell), `obs-scene.md`, `privacy-checklist.md` |
| `setup.sh` | reset the demo content to the committed state, print the one-time GUI steps |
| `prepare.sh` | macOS: quit, clean and reopen the editors on the demo content, place the windows |
| `rehearse.sh` | macOS: run one segment (or all) through Hammerspoon, report to `run/rehearsal.log` |
| `run/` | gitignored runtime files |

## Prerequisites (macOS)

```bash
brew install --cask obs
brew install ffmpeg displayplacer
```

Hammerspoon's command-line port, one line in `~/.hammerspoon/init.lua` (then *Reload Config*):

```lua
require("hs.ipc")
```

Everything else is already on the machine: Hammerspoon, Ghostty, Neovim + LazyVim, VS Code
with vscode-neovim, IntelliJ with IdeaVim, Obsidian, the daemon, and the keyboards repo.
KeyCastr is not needed: the HUD draws the typed keys itself and quits KeyCastr when it starts.

One-time GUI steps (printed by `setup.sh`): register `showcase/Demo` as an Obsidian vault
with Restricted mode off and Vim key bindings on, then `zmk-vim-mode install --obsidian` with
Obsidian quit; open `demo-java` in IntelliJ once and accept a JDK; trust the VS Code workspace.

## Run

```bash
bash showcase/prepare.sh            # clean editor state (also resets the demo content)
bash showcase/hud/start.sh          # HUD top-right + typed-keys strip bottom-left of the recording display
bash showcase/rehearse.sh all       # or one segment 3–8; VERBOSE=1 logs every keystroke with its time
bash showcase/hud/start.sh stop     # or the ✕ on the panel
```

Demo terminal (prompt shows only the directory, no history, big font):

```bash
ZDOTDIR=$PWD/showcase/env open -na Ghostty --args --config-file=$PWD/showcase/env/ghostty-demo.conf
```

Then follow `env/obs-scene.md` and `env/privacy-checklist.md`, and record one take per beat of
`SCRIPT.md`. On Omarchy use `linux/prepare.sh`, `linux/hud.sh`, `linux/rehearse.py`.

## Linux verification (Omarchy)

On 2026-09-15, verified the native layer-shell HUD on Hyprland 0.56.2, readable USB/Bluetooth
Diamond input devices, WebSocket mode delivery, and segment 3 (`pass=4 fail=0`). Arch packages:
`python-gobject webkit2gtk-4.1 gtk-layer-shell python-evdev python-websockets`.
The keymap builder accepts both Arch's Python `yq` and Mike Farah's `yq`.

```bash
bash showcase/linux/hud.sh
python3 showcase/linux/rehearse.py 3 --verbose
bash showcase/linux/hud.sh stop
```

`linux/panel.py` hosts both pages in transparent WebKitGTK layer-shell surfaces. The HUD reserves
a right rail (~617 px: 598 px panel + insets), keeping tiled editors beside it; the typed-keys
strip is stacked below the HUD inside that rail and reserves nothing, so the bottom of the
screen belongs to the editors again. The HUD is inset 8 pixels from the tiled client area's
top/right edges so the blue window border stays visible. The surrounding window canvas and
outlines are transparent; the HUD panel and typed-key chips retain their dark backgrounds,
and keycaps remain solid. The surfaces never take keyboard focus.

Stopping the host or using the HUD's close request releases the reserved area automatically.
No persistent Hyprland configuration is needed. Software WebKit rendering avoids a DMA-BUF
Wayland protocol error observed on this NVIDIA machine.

Editor preparation ran; segments 4–7 still need a fully passing run. Save existing editor work first:
`linux/prepare.sh` gracefully closes editor windows, including those on other projects, and
waits for them to close. It stops on unresolved save prompts rather than killing Electron's
child processes. VS Code workspace/recovery storage is preserved.

The demo editors use workspaces 5 (VS Code), 6 (IntelliJ), 7 (Obsidian), and 8 (Neovim/Bash), with Hyprland's
**maximized** mode. True fullscreen hides the HUD and bypasses panel reservations. Preparation
checks the resulting window state before reporting success; launcher output is kept in
`run/{code,idea,obsidian}.log`. The rehearsal also maximizes each target window, including Neovim's
demo terminal, within the reserved work area.

Linux rehearsals use Bash with `env/demo.bashrc` (isolated prompt/history). The demo terminal
keeps Ghostty's standard application class so the daemon recognizes it, and the runner selects
it by PID. Focus is verified before every keystroke; an unexpected focus change stops the run.
Segment 8 has also passed (`pass=4 fail=0`) with this Bash launcher.

## How the HUD works

The keymap on the HUD is the keyboards repo's own keymap-drawer description
(`docs/img/diagrams/keymap-drawer/keymap-drawer.yaml`), so it matches the published diagrams
key for key: 24 keys in `1333+2` per hand, layers `alpha1`, `alpha2`, `vim`, `numbers`,
`symbols`, `nav`, … and 148 combos with their positions. `build.py` checks the letter combos
against the documented ones (`ns=q`, `mg=k`, `st=w`, `cp=v`, `lo=x`, `ra=z`, `ae=y`, `h,=j`)
and gives the Enter and Tab combos their real layer coverage (`combos.dtsi`).

Two feeds drive it:

- **Vim layers — ground truth from the daemon.** One `msg=decision mode=… code=… reason="…"`
  line per transition (launchd's log file on macOS, the journal on Linux). The banner names the
  keyboard's layer: *VIM NORMAL* / *VIM VISUAL* show the `vim` drawer layer, *VIM INSERT* and
  *VIM CMDLINE* keep the vim tint while showing Alpha 1's legends (those layers are transparent
  on the keyboard), *RAW* and *ALPHA 1* have no vim layer; the daemon's reason is printed
  verbatim. The transitions the firmware itself performs (`i a o s c`→INSERT, `Esc`→NORMAL,
  `v`→VISUAL, `:` `/`→CMDLINE) move the banner instantly, dashed until the daemon confirms.
- **Keys — an emulation.** The host sees keycodes, not key positions, so `hud.js` resolves
  every keystroke against the active layer stack. Typing always goes through the two alpha
  layers: a letter not on Alpha 1 is the sticky Alpha 2 (one shot); the Alpha 1 letter combos
  are vim commands and are only considered while the vim layer is active. A key found on
  another layer means a held or one-shot layer the host could not see; the HUD shows it with
  its activator thumb and drops it after the next base-layer key or 700 ms. Combos draw a pill
  with the produced key and its meaning, linked to the keys that formed them.

Real per-key telemetry would need `CONFIG_ZMK_USB_LOGGING` on the dongle; the emulation is
enough for a scripted demo and needs no firmware change.

Rehearsing without the keyboard: `zmk-vim-mode set normal|insert|visual|cmdline|raw|off`
moves the banner (and the real keyboard); `hs -c "zmkhud.mode(2)"` moves only the HUD;
`hs -c "return table.concat(zmkhud.lastKeys, '\n')"` shows the last raw key events.

## Layout of the data

`hud/keymap.json` — `layout.keys[i]` gives each drawer index its hand, row, column and ZMK
position (`config.dtsi` names: LTR … R1); `layers[name][i]` is `{tap, hold, shifted, type}`;
`combos[]` carry `positions`, `key`, `layers`; `activators[]` say which key reaches which layer
and how; `codes` maps the daemon's 0–7 to a banner and a base layer.

| drawer idx | ZMK | keys |
|---|---|---|
| 0–2 / 3–5 | 1 2 3 / 6 7 8 | top row (ring, middle, index per hand) |
| 6–9 / 10–13 | 10–13 / 16–19 | home row (pinky → index / index → pinky) |
| 14–16 / 17–19 | 21 22 23 / 26 27 28 | bottom row |
| 20 21 / 22 23 | 31 32 / 33 34 | thumbs L1 L0 / R0 R1 |
