# Showcase video kit

Everything needed to record *Rafael Romão's keymap and supporting tools*: the script, committed
demo content with no private data, a one-command editor reset, and an automated rehearsal. The
layer HUD is not in here: it is its own project,
[zmk-layer-hud](https://github.com/rafaelromao/zmk-layer-hud), started by `hud.sh`.
`HANDOFF.md` is the long-form state of the work.

**The kit records on Omarchy Quattro (Arch + Hyprland).** The macOS half — the Hammerspoon host,
its runner and its recording notes — was removed when the HUD moved out; `git log` has it.

| Path | What |
|---|---|
| `SCRIPT.md` | the video script: beats, timings, narration, expected mode-line reason per take, shot list, pre-flight |
| `YOUTUBE.md` | title, thumbnail, description, chapters, where to post, the Short |
| `HANDOFF.md` | state, decisions and lessons for the next agent |
| `TAKE-2-PLAN.md` | review of the first take (2026-09-19) and the production plan for the second |
| `hud.sh` | start/stop the layer HUD for a take, from its own checkout (`$ZMK_LAYER_HUD`, default `~/projects/zmk-layer-hud`) |
| `prepare.sh` | quit, clean and reopen the editors on the demo content, maximized on their workspaces |
| `rehearse.py` | run one segment (or all) with synthesized keystrokes, report to `run/rehearsal.log` |
| `record.py` | screen-record every beat (0–9) hands-off, one segment each, with a TTS scratch narration → `run/take<N>.mp4` |
| `rehearsal-panel.py`, `rehearsal-feed.py` | the rehearsal's own copy of the HUD — see *Rehearsals* below |
| `setup.sh` | reset the demo content to the committed state, print the one-time GUI steps |
| `Demo/` | the Obsidian vault used on camera (open it as a vault; its runtime state is ignored) |
| `demo-go/`, `demo-go.code-workspace` | Go module for Neovim and VS Code |
| `demo-java/` | plain Java project for IntelliJ IDEA |
| `env/` | `ghostty-demo.conf` and `demo.bashrc` (demo terminal), `obs-scene.md`, `privacy-checklist.md`, `cards/` (title, channel, pipeline, node, links — printed with `show <card>`) |
| `run/` | gitignored runtime files |

## Prerequisites

The keyboard announces its own layers, so the HUD needs firmware support — already flashed on the
Diamond and the Rommana (`~/projects/keyboards/src/features/hud.dtsi`): the `zmk,layer-signal`
module with `positions;`, and `CONFIG_ZMK_HID_KEYBOARD_REPORT_SIZE=12` on the central/dongle
(HKRO, not NKRO). `zmk-layer-hud/docs/zmk-setup.md` is the step by step.

The HUD host, once:

```bash
git clone https://github.com/rafaelromao/zmk-layer-hud ~/projects/zmk-layer-hud
~/projects/zmk-layer-hud/bin/zmk-layer-hud setup --link
sudo pacman -S python-evdev             # the rehearsal only, see below
zmk-layer-hud config link ~/projects/zmk-layer-hud/config/diamond.yaml
zmk-layer-hud keymap                    # must convert with no error
```

`config/diamond.yaml` points at `~/projects/keyboards`; check those paths after copying it.
`python-evdev` is for the rehearsal only (`rehearsal-feed.py` reads the keys ydotool injects),
as is being in the `input` group.

The rest is already on the box: Hyprland with its Lua dispatchers (0.55+), OBS, ydotool +
`ydotoold`, Ghostty, Neovim + LazyVim, VS Code with vscode-neovim, IntelliJ with IdeaVim,
Obsidian, the daemon, and the keyboards repo. KeyCastr has no counterpart here and is not needed:
the HUD draws the keys the *keyboard* sends.

One-time GUI steps (printed by `setup.sh`): register `showcase/Demo` as an Obsidian vault with
Restricted mode off and Vim key bindings on, then `zmk-vim-mode install --obsidian` with Obsidian
quit; open `demo-java` in IntelliJ once and accept a JDK; trust the VS Code workspace.

## Run

```bash
bash showcase/prepare.sh            # clean editor state (also resets the demo content)
bash showcase/hud.sh                # the layer HUD on the recording monitor
python3 showcase/rehearse.py all    # or one segment 0–9; --verbose logs every keystroke
bash showcase/hud.sh stop           # or the ✕ on the panel
```

`bash showcase/hud.sh log` tails the HUD's panel and feed logs. The demo terminal (prompt shows
only the directory, no shared history, big font) is Bash with `env/demo.bashrc`; the command is
in the header of `env/ghostty-demo.conf`, and `rehearse.py` opens exactly that.

Then follow `env/obs-scene.md` and `env/privacy-checklist.md`, and record one take per beat of
`SCRIPT.md`.

## How the HUD works

It is a separate project, and it has one source: the keyboard. A small ZMK module writes
HID keyboard-page usages that no OS maps to a key (`0xC0 + layer id` for each active layer, `0xDF`
to commit) into the keyboard's own report on every layer change, and with `positions;` the
physical position of every press and release. `zmk-layer-hud`'s host reads those raw reports with
hidapi and draws the keyboards repo's own keymap-drawer description — the same file the published
diagrams come from. So the banner and the lit keys are ground truth, not an emulation: the exact
key lights, combos group by the keymap's own `COMBO_TERM`, held and one-shot layers appear because
the keyboard says so.

The banner names the top active ZMK layer through the config's `layers.map` (`VIM_NORMAL` →
*Vim normal*, `VIM_INSERT` → *Vim insert*, `ALPHA1` → *Alpha 1*) and lists the whole active set
below it, so visual mode reads *Vim visual · Vim normal*.

Two things it does **not** show, both worth knowing before a take:

- **No daemon reason in the HUD.** The daemon is not a source any more. For the takes, the
  *mode line* — a pinned terminal in the rail running `watch -n 0.2 -t 'zmk-vim-mode status |
  head -1'` (`env/obs-scene.md`) — puts the reason on camera beside the board.
- **RAW and OFF look the same.** `raw` (code 6) selects no vim layer, so it reads *Alpha 1*, just
  like code 0 — and codes 4 and 7 (legacy) read *Vim normal*, just like code 1 (see
  `keyboards/src/features/vim.dtsi`). Beat 8 of `SCRIPT.md` is about exactly that distinction, so
  it is carried by the mode line, not by the banner.

The layout, the ZMK position of each drawer key, the layer table and the combo coverage all live
in `~/.config/zmk-layer-hud/config.yaml` (`positions:`, `layers.map`, `combos:`); that project's
README documents every key. Nothing in this repo generates HUD data any more.

## Rehearsals

`rehearse.py` types with ydotool, which writes to `/dev/uinput` and never reaches the Diamond — so
those keys are invisible to a HUD that reads the keyboard. The rehearsal therefore runs its own
copy of the HUD: `rehearsal-panel.py` puts zmk-layer-hud's pages in the same layer-shell surfaces
on port 8767, and `rehearsal-feed.py` runs that project's own reader (the keymap, the device, and
the layers the keyboard really is on) and adds the injected keys read off ydotool's virtual
device. The pages light them against the real layer stack, exactly as they light real typing.

`zmk-vim-mode set normal|insert|visual|cmdline|raw|off` moves the banner in a rehearsal *and* in a
take, because the daemon writes the mode to the physical keyboard and its layers actually change.

Stop the take's HUD first: both anchor to the same corner and reserve the same rail, so
`rehearse.py` refuses rather than stacking them. `NO_HUD=1 python3 showcase/rehearse.py 4` skips
the rehearsal HUD altogether; the daemon expectations do not depend on it. Logs:
`run/rehearsal-panel.log` and `run/rehearsal-feed.log`; the report is `run/rehearsal.log`.

Keep the rehearsal HUD's surfaces in step with `$ZMK_LAYER_HUD/host/linux/panel.py` if that one
changes: `rehearsal-panel.py` is a copy of the panel it was ported from.

## Recording notes

The editors run **maximized**, never fullscreen: fullscreen hides the layer-shell HUD and ignores
its reservations. The HUD reserves a right rail (~617 px: the 598 px panel plus insets), keeping
tiled editors beside it; the typed-keys strip sits below the HUD inside that rail and reserves
nothing, so the bottom of the screen belongs to the editors. The surfaces never take keyboard
focus. Stopping the host, or the HUD's ✕, releases the reservation. No persistent Hyprland
configuration is needed.

The demo editors use workspaces 5 (VS Code), 6 (IntelliJ), 7 (Obsidian) and 8 (Neovim/Bash).
`prepare.sh` gracefully closes editor windows, including those on other projects, and waits for
them — save your work first. It stops on unresolved save prompts rather than killing Electron's
child processes; VS Code workspace/recovery storage is preserved, and launcher output is kept in
`run/{code,idea,obsidian}.log`.

The rehearsal verifies focus before every keystroke and stops on an unexpected focus change. It
maximizes each target window, including the demo terminal, inside the reserved work area.

Earlier verification on the previous Omarchy box (2026-09-15, Hyprland 0.56.2): the layer-shell
surfaces, readable USB/Bluetooth Diamond input devices, and segments 3 and 8 (`pass=4 fail=0`
each); segment 4 was 26 pass / 1 fail. Those numbers predate both the move to zmk-layer-hud and
Omarchy Quattro — re-run them there before trusting them. Software WebKit rendering
(`WEBKIT_DISABLE_DMABUF_RENDERER=1`) avoids a DMA-BUF Wayland protocol error seen on the NVIDIA
machine, and is set by both panels.
