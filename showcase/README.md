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
| `narration.py` | parses a beat's narration out of `SCRIPT.md`, cues included — shared by `record.py` and `dub.py` |
| `dub.py` | post: render the narration, place each cued line on its moment, normalise, mux → `run/showcase-dubbed.mp4` |
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

## Dubbing

`record.py` lays a TTS scratch under each take for judging pace. `dub.py` produces the
deliverable, and unlike the rest of the kit it needs no Hyprland, no `ydotool` and no
recorder — it is ffmpeg plus a TTS binary, so the dub can be cut on a laptop while the
takes are recorded on the Linux box.

```bash
python3 showcase/dub.py anchors   # measure each take's first on-screen action
python3 showcase/dub.py hud       # what layer the HUD shows, when
python3 showcase/dub.py keys N    # the typed-keys strip through take N
python3 showcase/dub.py fit       # does the narration fit the picture? (no TTS needed)
python3 showcase/dub.py render    # render every cued line
python3 showcase/dub.py master    # lay the bed, normalise, mux onto the assembly
python3 showcase/dub.py pauses    # find the stretches that are both silent and frozen
python3 showcase/dub.py tighten   # cut those out, re-place the narration
python3 showcase/dub.py check     # sync, density and loudness of the result
```

`dub.py resync` skips the TTS and rebuilds the bed from the existing `run/take<N>.wav`
scratch files — useful to fix the timing of an assembly you already have.

**Cues.** A narration line in `SCRIPT.md` may open with `[+12.3]`: speak this line 12.3 s
after *the take's first on-screen action*, not 12.3 s into the file. The takes open with a
few seconds of nothing, because the recorder starts before the segment does, and that
lead-in is not the same in every take — 5.2 s in takes 0–3, 3.9 s in takes 4–9. `dub.py`
measures it per take, so the script never has to know. A line with no cue is simply joined
onto the line above it, which is how every beat read before cues existed.

**Why per-line cues.** One clip per beat drifts against the picture as soon as the beat has
more than one thing happening in it — beat 4 has eleven. Each cue is placed at its absolute
position in the assembly, so a clip that renders long can never push the ones after it.

**A take opens with the previous beat's screen.** The recorder starts several seconds
before the segment does, the HUD rail is drawn at 3.7-5.2 s, and the take's own picture
does not arrive until about 10 s. Everything in between is whatever the beat before it
left on the monitor. `dub.py anchors` therefore reports two numbers: `action`, the rail
appearing, which is what cues are relative to; and `content_start`, when this take's
picture actually begins. **No beat's opening line may start before `content_start`** — if
it does, it narrates the previous beat over the previous beat's screen. Getting this wrong
put every one of the ten beats 5-6.6 s early, and it survived several rounds of checking
because each beat was internally consistent: only the relationship to the picture was off.

Note that cropping the rail away does not rescue full-frame scene detection here — the
editor window resizes when the rail appears, so the content pane changes too.

**Never sample these recordings with `-ss` before `-i`.** They carry sparse keyframes and a
nominal 60 fps that is really 59.99 (`nb_frames` is six short of `duration × 60`), so input
seeking lands on a keyframe some seconds away and returns that frame without a word of
complaint. Sample with the `fps` filter in a single decode — `dub.py keys` does — or with
`-ss` *after* `-i`. This is not a nicety: it is what put beat 4's cues eight seconds out and
then made the frame-by-frame check that should have caught it agree with them. Two rounds of
"it is still out of sync" came from trusting a seek.

**The banner is not enough on its own.** `dub.py hud` says which layer is lit, and that is
the right anchor for a line about a *mode* — insert, normal, raw. It is the wrong anchor for
a line that names a *keystroke*, because the banner reads `normal` for twenty seconds while
the segment types an entire tour. Cue those against `dub.py keys`, which stacks the HUD's
typed-keys strip through a take so you can read what was actually being pressed and when.
Beat 4 is the cautionary one: every line sat on the correct layer and four of them were
still four to ten seconds adrift, two of them describing an action that had already
happened. Its cue list now carries the keystroke timeline as a comment.

**Place cues against `dub.py hud`, never against a guess.** The HUD banner's underline is a
solid bar whose colour *is* the layer — green for insert, bright blue for the vim command
layers, dim blue-grey for Alpha 1 — so cropping it and averaging it to one pixel turns "what
was the keyboard doing" into three bytes per sample. Scene detection is not a substitute:
on the whole frame it only says *something* moved, and on the banner it misses
insert→normal entirely, because the text is the same length and only the colour changes.
`dub.py fit` prints the layer each line is spoken over, so a line that talks about the vim
layer while the HUD reads Alpha 1 is visible before anything is rendered.

**Voice.** `dub.py` tries kokoro-onnx first, then piper, both from `~/.cache/zmk-showcase`:

```bash
python3.12 -m venv ~/.cache/zmk-showcase/tts-kokoro
~/.cache/zmk-showcase/tts-kokoro/bin/pip install kokoro-onnx soundfile numpy
# then kokoro-v1.0.onnx + voices-v1.0.bin into ~/.cache/zmk-showcase/voices/
```

`ZMK_DUB_VOICE` picks the voice (default `am_michael`), `ZMK_DUB_SPEED` its pace,
`ZMK_DUB_LEAD` how far ahead of its action a line starts (default 0.3 s).

**Cutting the dead air.** The segments run longer than their words on purpose, so the
assembly carries a lot of silence — 608 s of picture under 415 s of narration. `dub.py
tighten` removes the worst of it, but silence on its own is not the test: a pause while
the segment is typing is the demo, not dead air. A stretch is only cut when nothing is
being said *and* the picture is not moving, and each candidate is then re-checked by
comparing the frame at its start with the frame at its end — if they differ, something
happened in there and the cut is dropped. That check is what saves the Diamond photo at
the end of beat 1, which is silent and nearly still but not still enough.

`ZMK_DUB_STILL` (0.6) is how frozen counts as frozen, `ZMK_DUB_MINGAP` (1.2 s) how long a
pause has to be before it is worth cutting, and `ZMK_DUB_KEEPGAP` (0.4 s) how much of it
survives so the cut does not feel abrupt.

**Delivery.** The bed is normalised to −16 LUFS with a −1.5 dBTP ceiling and delivered at
48 kHz stereo, per `env/obs-scene.md`. `master` muxes the picture `-c:v copy` and never
re-encodes it; `tighten` has to re-encode, because the cuts are not on keyframes.
`dub.py check` reports the sync error per beat.


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
