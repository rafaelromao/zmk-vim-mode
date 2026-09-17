# Showcase video — handoff

For the agent continuing this work on Omarchy Quattro (Arch + Hyprland), which is now the only
platform the kit supports. Started on macOS on 2026-09-15 after a full day of building and
rehearsing with the user; the macOS half was removed on 2026-09-16 (see below). Everything under
`showcase/` is described here; read `README.md` for the day-to-day commands and `SCRIPT.md`
for the video itself.

## What the deliverable is

A 6–8 minute English voice-over video, *"Rafael Romão's keymap and supporting tools"*: the
Diamond (24 keys, `1333+2`), the Magic Romak layout, and zmk-vim-mode keeping the keyboard's
layers in sync with the editor's vim state in Neovim, VS Code, IntelliJ IDEA and Obsidian.
The script (`SCRIPT.md`) has ten beats with timings, narration (952 words), the on-screen
actions and the daemon `status` reason each take must show.

The recording is done by the user, on the Diamond, on whichever machine records best. The
kit's job is to make every take predictable: a layer HUD on screen, clean demo content, a
one-command reset of the editors, and an automated rehearsal that proves the daemon reports
the expected state at every step before anyone presses record. The HUD itself is no longer part
of the kit: it is [zmk-layer-hud](https://github.com/rafaelromao/zmk-layer-hud).

## State at handoff

### HUD replaced, kit is Linux only — 2026-09-16

- **The HUD moved out.** `hud/` (pages + emulator + Hammerspoon host), `keymap/build.py` and
  `linux/{keyfeed.py,hud.sh}` are gone; `bash showcase/hud.sh` starts
  [zmk-layer-hud](https://github.com/rafaelromao/zmk-layer-hud) from `$ZMK_LAYER_HUD` (default
  `~/projects/zmk-layer-hud`). **HUD work happens in that repo — do not vendor it back here.**
- Why it is better: the keyboard announces its active layers and every key position inside its
  own HID reports (`keyboards/src/features/hud.dtsi`), so the board is lit from ground truth
  instead of the old emulator's guess at which key produced a character.
- What it costs, and it matters for the script: there is **no daemon reason on screen**, and
  `raw` (code 6) selects no vim layer, so **RAW and OFF both read *Alpha 1*** — as codes 4 and 7
  read *Vim normal*, like code 1 (`keyboards/src/features/vim.dtsi`). Beat 8 was re-staged around
  `zmk-vim-mode status` in the demo terminal. In exchange, visual mode is now distinguishable
  (*Vim visual · Vim normal*) and combos/one-shots are exact.
- **Rehearsals keep their picture.** ydotool writes to `/dev/uinput` and never reaches the
  Diamond, so those keys are invisible to a feed that reads the keyboard. `rehearse.py` therefore
  starts `rehearsal-panel.py` (zmk-layer-hud's pages in the same layer-shell surfaces, port 8767)
  with `rehearsal-feed.py`, which runs that project's own reader — keymap, device, and the layers
  the keyboard really is on — and adds the injected keys read off ydotool's virtual device. The
  pages light them against the real stack. `NO_HUD=1` skips it.
- Why a second panel at all: zmk-layer-hud's Linux host spawns its own `hudfeed.py` and quits
  when that dies, and one variable (`ZMKHUD_PORT`) sets both the feed's port and the pages'
  `?ws=` URL — so it cannot be pointed at another feed without editing that repo. `rehearsal-panel.py`
  is a copy of the panel zmk-layer-hud was ported *from*; keep its surfaces in step with
  `$ZMK_LAYER_HUD/host/linux/panel.py`. Five lines there (a `--no-feed` flag and a page port
  separate from the feed port) would let this copy be deleted — the user's call.
- **macOS dropped, at the user's request.** `prepare.sh`/`rehearse.sh` for Hammerspoon,
  `hud/{hud,place,rehearse}.lua` and the macOS recording notes were removed; `linux/` was
  flattened into `showcase/` (`prepare.sh`, `rehearse.py`). `env/.zshrc` should go too — the demo
  shell is Bash with `env/demo.bashrc`, and `env/ghostty-demo.conf` no longer forces zsh.
- `keymap/build.py`'s combo sanity checks (`ns=q`, `mg=k`, `st=w`, `cp=v`, `lo=x`, `ra=z`,
  `ae=y`, `h,=j`) went with it. They were really assertions about `~/projects/keyboards`; if they
  are wanted back, that is where they belong. The Enter/Tab layer coverage it used to patch is
  now the `combos:` block of `~/.config/zmk-layer-hud/config.yaml`.
- New prerequisites, all in `README.md`: the firmware module (already flashed), the HUD repo's
  venv, `~/.config/zmk-layer-hud/config.yaml`, the hidraw udev rule, `python-evdev` and the
  `input` group for the rehearsal.
- **Nothing here has run on Omarchy Quattro yet.** The dated results below are from the previous
  box and the old HUD.

### Omarchy continuation, 2026-09-15

*(the HUD described in this section is the kit's old one; zmk-layer-hud inherited the layer-shell
work from it almost verbatim, which is why its Linux panel behaves the same)*

- Daemon doctor: 15 checks passed, no warnings; Diamond writable on USB and Bluetooth.
- Installed `python-evdev` and `python-websockets`; Diamond input devices are readable.
- Fixed the builder for Arch's Python/jq `yq` (its `4.1.2` version is not Mike Farah v4).
  Build passes: 24 keys, 22 layers, 148 combos, 35 activators.
- Added Lua runtime window rules for Hyprland 0.56.2; Chromium Wayland app windows use
  generated classes despite `--class`. Both overlays verified floating/pinned at their
  requested sizes and positions; WebSocket sends daemon mode. HUD left running.
- Linux segment 3: **4 PASS, 0 FAIL**, override cleared afterward.
- Still pending: physical key-light/close-button confirmation, fully passing segments 4–7,
  and the IntelliJ issues below.
- User closed VS Code and authorized preparation. Preparation ran, but the first segment 4
  exposed Linux launcher/focus issues and was interrupted while the vault was being configured.
- Fixed the runner for Lua Hyprland focus commands, verified window addresses before typing,
  and made reason mismatches fail checks. Ghostty now uses its standard class (daemon-recognized)
  and is selected by PID. User requested Bash: Linux uses `env/demo.bashrc`; macOS keeps Zsh.
  Zsh was installed during diagnosis before the Bash request, but Linux rehearsals no longer need it.
- Segment 8 with Bash: **4 PASS, 0 FAIL**. Segment 4 with Bash: **26 PASS, 1 FAIL**;
  only the short leader-pending RAW check failed. Shortened its wait to account for the
  character delay. Verification retry stopped before typing because demo focus was not held;
  that timing change still needs verification in a hands-off run.
- User requested HUD alignment with blue borders: verified top **44**, right **2047** logical
  pixels on this monitor. Startup derives those edges from tiled windows rather than a fixed margin.
- Superseded Chromium with `linux/panel.py` (WebKitGTK + GTK layer shell) after the user requested
  transparent backgrounds and a bottom panel. Installed `gtk-layer-shell`; GObject/WebKitGTK
  were already available. Both page backgrounds and typed-key chips are transparent; keycaps
  stay solid. HUD now sits **8 px inside the client edges**, leaving the blue border visible
  (top 54, right inset 11 on this monitor). Verified visually with a screenshot.
- Typed keys now occupy a **96 px exclusive bottom panel**, verified monitor reserved area
  `[0,43,0,96]` and master client height 1007. Close protocol releases the reservation; restarting
  restores it. HUD left running. No persistent Hyprland config edits. WebKit DMA-BUF rendering
  is disabled by default to avoid an observed NVIDIA Wayland protocol error.
- User clarified transparency: only the surrounding white window areas/outlines should be
  transparent. Restored the original dark HUD panel and typed-key chip backgrounds; retained
  the transparent native canvas, bottom reservation, and HUD inset.
- Added a full-height right rail (`zmkhud-reserved`) so the HUD reserves space too:
  monitor reserved `[0,43,617,96]` (top bar, right rail, bottom panel), bottom strip spans
  the editor width only. Restarted HUD to activate it and re-verified segment 3 (4/4 PASS).
- Moved the typed-keys strip below the HUD inside the right rail (same 598 px width, 8 px gap)
  and reclaimed the bottom: monitor reserved `[0,43,617,0]`, keys overlay at y=454 with no
  exclusive zone. Fixed the demo terminal theme case (`tokyonight` → `TokyoNight`, matching
  `/usr/share/ghostty/themes`) to silence the theme error popup on launch.
- Fixed `doctor` Obsidian warning via `zmk-vim-mode install --obsidian`: Demo vault plugin
  now installed and enabled. Remaining warning (`editor clients none connected`) is expected
  with no editor open. Doctor: 0 failed, 1 warning.

### Layout/reset correction, 2026-09-16

*(before the HUD was replaced; the reservations behave the same, so all of it still holds)*

- Use **maximized**, not fullscreen: fullscreen hides the layer-shell HUD and ignores its reservations.
- User assigned demo workspaces starting at **5**: VS Code 5, IntelliJ 6, Obsidian 7,
  Neovim/Bash 8. Preparation switches workspace before launching and verifies maximized state.
- Removed broad `pkill` calls from preparation. VS Code logs showed renderer/utility exits with
  code 15 immediately before the SIGTRAP core; the reset killed subprocesses while the app was alive.
  Graceful window closure now waits, and VS Code workspace/recovery storage is preserved.
- Closed three leftover `hyprland-dialog` application-not-responding dialogs by their verified PIDs.
- Latest preparation verified VS Code briefly, then stopped because no demo-java window appeared.
  Full editor startup/stability remains unverified. No editor windows remained at the last check;
  do not treat script messages alone as proof of a stable layout.

### AFK pause, 2026-09-16 ~11:38 -03 — recording environment lost

*(paths below updated for the flattened layout: `showcase/prepare.sh`, `showcase/hud.sh`,
`showcase/rehearse.py`)*

- External Dell display disconnected: only a FALLBACK 1920x1080 output remains, reserved
  `[0,43,0,0]` (top bar only). The HUD host is still running but bound to the gone HDMI-A-1;
  restart it with `bash showcase/hud.sh` once the recording display is back.
- Diamond USB side gone (keyfeed lost its evdev node; doctor lists only the Bluetooth endpoint).
  No Diamond evdev nodes readable, so typed-key lighting has no input. Reconnect USB/BLE dongle.
- All demo editor windows/processes gone (VS Code, IntelliJ, Obsidian). Daemon healthy:
  segment 3 re-verified 4/4 PASS, doctor 0 failed / 1 warning (no editors, expected).
- On return: re-dock display, reconnect keyboard, `bash showcase/prepare.sh`,
  `bash showcase/hud.sh`, then `python3 showcase/rehearse.py all` hands-off.
  Segments 5–7 still have no full pass (seg5 reached the terminal-toggle step on ydotool).

### Take-HUD setup, 2026-09-17 (ran on the box)

- Prereqs that were already satisfied: system `python-gobject webkit2gtk-4.1 gtk-layer-shell
  python-evdev`, Diamond on `/dev/hidraw0` (USB `0003:1D50:615E`), the hidraw udev rule (identical
  to the daemon's — no sudo needed), firmware already flashed.
- `make venv` in `~/projects/zmk-layer-hud` + `~/.config/zmk-layer-hud/config.yaml` (copied from
  `config/diamond.yaml`; keyboards paths check out) + keymap converts (24 keys, 148 combos).
- **Gotcha, reported upstream, not fixed there:** pip's `hidapi` builds the hidraw backend as a
  *separate* `hidraw` module; `import hid` is libusb-only, enumerates no hidraw paths and cannot
  open a kernel-bound keyboard (`cannot open ? (b'1-5.1.1:1.0')`). The venv's `hidraw` module
  opens the Diamond fine. Workaround in use: `ZMKHUD_PYTHON=/usr/bin/python3` (Arch's
  `python-hidapi`, whose `hid` has the hidraw backend) — so `bash showcase/hud.sh` currently
  depends on that env var. Durable options for Rafael: `hudfeed.py` prefers `import hidraw`
  (one line), or the Makefile asserts a hidraw path post-venv. Rebuilding the venv's hidapi
  from source (`pip install --force-reinstall --no-binary hidapi`, gcc + libudev.h present)
  does NOT help — same split modules.
- Verified live: `zmk-vim-mode set normal` → banner *Vim normal* (screenshot); daemon stays
  `writable` on USB+BT while the feed reads (open issue 3 closed on the daemon side); physical
  Diamond presses arrive as `{"kind":"press","pos":16/19}`. Override cleared afterwards
  (daemon back to auto/off). Key *lighting* on camera still needs eyes, not logs.
- Rehearsal side untouched: `rehearsal-feed.py`/`rehearsal-panel.py` still **not yet run**.

### Corrections, 2026-09-17 (agent got these wrong)

- **No custom keybindings are needed — reverted.** `seg5` toggles the terminal through
  `palette("View: Toggle Integrated Terminal")`, exactly as upstream wrote it. The temporary
  `~/.config/Code/User/keybindings.json` (F20, then F12 bindings) is deleted; the profile is
  back to stock. The `F20`/`F12` detour and the full a–z `KEYCODES` table are reverted out of
  `rehearse.py`. What the evidence actually supports, kept as comments: keycode 41 (grave)
  arrives fine at kernel level (`showkey`: 96 0140 0x60) but this box's VS Code never fires its
  default Ctrl+` binding from uinput-injected chords; keycode 190 never leaves ydotool's virtual
  device (`showkey` stays silent). The recording takes press the real chord on the Diamond and
  are unaffected either way.
- **IntelliJ was not blocked.** The user confirmed `demo-java` opens with the correct project;
  the Welcome-screen / "Cannot Execute Command" / instant-dispose episodes were a stale
  windowless instance being reused plus slow startup, not a broken project. `prepare.sh` now
  waits for the `idea` process itself to quit before relaunching (committed).
- **The new HUD's firmware is already flashed** (prerequisites in `README.md`); there is no
  reflash unknown. What remains is host setup + a live run: `make venv` + websockets in
  `~/projects/zmk-layer-hud`, the hidraw udev rule, `~/.config/zmk-layer-hud/config.yaml`,
  then `bash showcase/hud.sh` with the Diamond typing.
- **Old HUD orphans stopped.** `showcase/linux/panel.py` + `keyfeed.py` were still running after
  the merge deleted those paths; both processes are terminated. Nothing HUD-related runs now.
- **Demo content on disk was always clean.** Every rehearsal failure left edits in VS Code
  buffers only; they were discarded with `:q!` + quick-open reopen (verified pristine tab, no
  dot), and `git status` on the demo dirs came back clean each time. The editors are currently
  closed by the user with the right files last open.
- **Uncommitted rehearsal fixes in this tree** (beyond upstream): `palette()` dismisses any open
  palette with Escape before F1 (the palette retains input across invocations; BackSpace-clearing
  must NOT be used — it eats the `>` prefix and drops to file-search). `seg5` close-toggle is the
  current frontier: open-toggle proven twice, close-toggle not yet green in a full run.

### Where each piece stands

| Piece | Status |
|---|---|
| the layer HUD | now `zmk-layer-hud`; its own README carries its status. Its Linux panel had not been run on hardware when it was adopted |
| `hud.sh` | **ran 2026-09-17 on Omarchy Quattro** (see HUD setup notes below); take HUD verified: banner follows `zmk-vim-mode set`, physical Diamond presses arrive with positions |
| `rehearsal-panel.py` + `rehearsal-feed.py` | written 2026-09-16, **not yet run**. The panel is the old `linux/panel.py` (verified 2026-09-15 with the old pages) repointed at zmk-layer-hud's pages on port 8767; the feed is new |
| `prepare.sh` (was `linux/prepare.sh`) | ran on the previous box; editors maximized on workspaces 5/6/7. Full editor startup/stability still unverified |
| `rehearse.py` (was `linux/rehearse.py`) | segment 3: 4 PASS / 0 FAIL; segment 8: 4 PASS / 0 FAIL; segment 4: 26 PASS / 1 FAIL (the leader-pending RAW check, since shortened, needs a hands-off re-run). Segments 5–7 never fully passed |
| `setup.sh` | git-based content reset; works |
| `SCRIPT.md` | beat 8 re-staged for the new HUD 2026-09-16; not re-timed on camera |

Open issues the user reported that are not resolved:

1. **IntelliJ shows an "IDE error" popup** at some point during the rehearsal. The user did not
   send the exception yet. Check `idea.log` (Help → Show Log) for the last `ERROR`; if the trace
   mentions `dev.rafaelromao.zmkvimmode`, it is our plugin (`editors/intellij`), otherwise it is
   IdeaVim/platform noise from synthesized keys.
2. **IdeaVim's `:` reports `raw`, not `cmdline`.** `editors/intellij/README.md` promises cmdline.
   The ex-entry field is a separate component, so the plugin's focus listener wins over the mode
   change. Either fix the plugin (treat the ex-entry focus as cmdline) or fix the README. The
   script currently documents the observed behaviour.
3. **The daemon and the HUD both hold the Diamond's HID device** — the daemon writes LED output
   reports, the HUD's feed reads input reports — and the two have never been observed running at
   once. First thing to check on the box: `zmk-vim-mode devices` while the HUD runs must still
   say `writable`, and `zmk-vim-mode set normal` must still move the banner.
4. `env/.zshrc` is still in the tree: the sandbox this was done from refuses to write shell rc
   files. `git rm showcase/env/.zshrc` by hand.

## Layout of `showcase/`

```
SCRIPT.md               the video script (beats, narration, actions, expected reasons, shot list)
README.md               how to run the kit
HANDOFF.md              this file
hud.sh                  start/stop zmk-layer-hud for a take ($ZMK_LAYER_HUD)
rehearsal-panel.py      the rehearsal's copy of the HUD: its pages in layer-shell surfaces, port 8767
rehearsal-feed.py       that project's reader + the keys ydotool injects (evdev)
prepare.sh              quit/clean/reopen/maximize the editors on their workspaces
rehearse.py             run a segment with ydotool; report to run/rehearsal.log
setup.sh                content reset (git checkout of Demo, demo-go, demo-java) + GUI steps
Demo/                   Obsidian vault (notes, attachments, .obsidian settings; runtime state ignored)
demo-go/                Go module for Neovim and VS Code (modes table); demo-go.code-workspace
demo-java/              plain Java project for IntelliJ (.idea/ ignored)
env/                    ghostty-demo.conf + demo.bashrc (demo terminal), obs-scene.md, privacy-checklist.md
run/                    gitignored runtime: rehearsal.log, rehearsal-{panel,feed}.log, shell history
```

## How the HUD works

One source: the keyboard. `keyboards/src/features/hud.dtsi` adds zmk-layer-hud's ZMK module, which
writes reserved HID keyboard-page usages into the keyboard's own report on every layer change
(`0xC0 + layer id` per active layer, `0xDF` to commit) and, with `positions;`, the physical
position of every press and release. No OS maps those usages to a key, so only a raw-HID reader
sees them. `zmk-layer-hud/host/hudfeed.py` decodes them and draws the keyboards repo's
keymap-drawer YAML with the drawer's own generators. Everything about the channel, the wire
format and the config is documented in that repo (`README.md`, `docs/zmk-setup.md`); nothing in
this repo generates HUD data any more.

Two consequences for the script, repeated here because they are easy to trip over:

- there is **no daemon reason on screen** — `zmk-vim-mode status` is the only judge of a take;
- **RAW ≡ OFF ≡ *Alpha 1*** and codes 4/7 ≡ 1 ≡ *Vim normal*, because `vim.dtsi` gives `raw` no
  layers and gives both legacy codes `VIM_NORMAL`.

Per-layer labels, the ZMK position of each drawer key, the combo coverage and every size and
timing live in `~/.config/zmk-layer-hud/config.yaml` (copied from that repo's
`config/diamond.yaml`).

## The box

The daemon side is Linux-first (that was the v1 target): systemd user unit, hidraw, udev rule
`contrib/udev/60-zmk-vim-mode.rules`, Hyprland focus backend, AT-SPI2 for VS Code's widgets. What
the kit adds on top:

| Concern | How |
|---|---|
| Layer HUD | `bash showcase/hud.sh` → `$ZMK_LAYER_HUD/host/linux/hud.sh`: layer-shell surfaces (`zmkhud-layer`, `zmkhud-keys`, `zmkhud-reserved`), raw HID from the keyboard, no key-event or daemon feed at all |
| Rehearsal HUD | `rehearsal-panel.py` + `rehearsal-feed.py` on port 8767 (see the 2026-09-16 notes) |
| Typing in rehearsals | `ydotool` via `/dev/uinput` (`wtype`'s virtual-keyboard events are dropped by VS Code); chords from the `CHORDS` table |
| Focus / placement | `hyprctl` Lua dispatchers (0.55+), addresses verified before every keystroke, **maximized** windows on workspaces 5–8 |
| Mouse click (Obsidian back-to-note) | `hyprctl dispatch movecursor` + `ydotool click 0xC0` or `wlrctl pointer click left` |
| Editor resets | `prepare.sh`: graceful window close, per-project UI state cleared, relaunch and maximize |
| Shortcuts in segments | Ctrl-chords (Ctrl+P, Ctrl+Shift+E, Ctrl+`, Ctrl+Shift+N, Alt+F12, Ctrl+E, Ctrl+Shift+F); Meh+B and Meh+G come off the keymap |
| Demo terminal | `ghostty --config-file=env/ghostty-demo.conf … -e /bin/bash --noprofile --rcfile env/demo.bashrc -i`, standard class so the daemon recognizes it, selected by PID |
| Display | Hyprland monitor config; OBS screen capture through the PipeWire portal |

Things to verify first on the box, in this order:

1. `zmk-vim-mode status` / `doctor` green; the Diamond `writable` over USB or BLE.
2. In `$ZMK_LAYER_HUD`: `.venv/bin/python3 host/keymap.py` converts the configured YAML.
3. `bash showcase/hud.sh`, then type on the Diamond → the exact keys light; then
   `zmk-vim-mode set normal` → banner *Vim normal*. `bash showcase/hud.sh log` if not. Likely
   first problems: hidraw permissions (the udev rule), or the firmware not announcing (9-byte
   reports mean `CONFIG_ZMK_HID_KEYBOARD_REPORT_SIZE=12` is missing).
4. `zmk-vim-mode devices` **while the HUD runs** (open issue 3).
5. `bash showcase/prepare.sh`, then `python3 showcase/rehearse.py 3 --verbose`, then `4`, then
   the rest — checking that the board and the strip light for the injected keys.
6. Obsidian must be the native package, not Flatpak (the plugin cannot reach the socket from
   Flatpak); the vault to register is `showcase/Demo` (folder name = vault name `Demo`, used in
   the `obsidian://` URIs).

## Lessons that cost time (do not repeat)

- Java apps ignore synthesized modifier chords from the host; use the Mehs keymap's chords
  (`Meh+B` project window, `Hyper+U` find action), which the firmware emits itself.
- IdeaVim rings the bell on `Esc` in normal mode: never send a redundant `Esc` there
  (`set visualbell` in `~/.ideavimrc` silences it for the recording).
- In VS Code's Explorer a letter creates a file in this user's setup; use arrows there.
- LazyVim: leader-pending `raw` lasts only `timeoutlen`+50 ms (~350 ms); the picker input is
  insert mode; leave `:terminal` with `Ctrl-\ Ctrl-n`, two Escapes race the mapping.
- Enter/Tab are combos on almost every layer (`cb_enter` RHM RHR R0, `cb_tab` RTM RTR R0) even
  though the drawer YAML lists them only under `shortcuts`; the `combos:` block of the HUD config
  gives them their real coverage.
- Everything typed in a tool window should start with a letter that is a home-row motion on the
  vim layer (`readme`, `keyboard`, `layer`, `ls`) so the RAW state reads on camera.
- Editors keep state between runs (dirty files, panels, reading view, tool windows) that changes
  what the daemon reports: reset once with `prepare.sh` rather than sending close commands
  mid-run (those caused their own beeps and errors).
- Claude Code's sandbox on the Mac could not reach the daemon socket, network sockets, nested
  `.git` dirs or dotfiles (it refused even to delete `env/.zshrc`); the user ran the scripts and
  pasted output, the agent read `run/rehearsal.log` and the daemon log. Expect similar limits.

## Suggested next steps

1. On Omarchy Quattro, walk the six checks above: nothing in the 2026-09-16 change has run yet.
2. `bash showcase/prepare.sh` + `python3 showcase/rehearse.py all`; fix what the move broke.
3. Settle the two IntelliJ items above (error popup, cmdline vs raw).
4. Record beats 3–8 one take each following `SCRIPT.md`; beats 0–2 and 9 are title cards,
   diagrams from `~/projects/keyboards/docs/img/diagrams/` and a pre-rendered terminal.
5. Decide whether zmk-layer-hud should grow a `--no-feed` flag so the rehearsal panel here can go.

## Sources of truth

- Keymap: `~/projects/keyboards` (`docs/img/diagrams/keymap-drawer/keymap-drawer.yaml`,
  `src/definitions/config.dtsi`, `src/features/vim.dtsi`, `src/features/combos.dtsi`).
- Daemon protocol and reasons: `internal/proto/proto.go`, `internal/state/`, the editor READMEs
  under `editors/`.
- The plan this was executed from: the "Context" and "Benefits analysis" sections are
  reproduced in `SCRIPT.md`'s argument paragraph and beats 2–3.
