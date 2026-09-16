# Showcase video — handoff

For the agent continuing this work from the Omarchy box (Arch + Hyprland). Written on macOS
on 2026-09-15 after a full day of building and rehearsing with the user. Everything under
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
the expected state at every step before anyone presses record.

## State at handoff

### Omarchy continuation, 2026-09-15

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

### Original macOS state

Verified on macOS (Rafael's MacBook, Diamond over USB, external 27" as recording display):

| Piece | Status |
|---|---|
| `keymap/build.py` → `hud/keymap.json` + `hud/keymap.js` | works; combo sanity checks pass; Enter/Tab combos given their real layer coverage from `combos.dtsi` |
| `hud/index.html` + `hud.js` + `hud.css` (layer HUD) | works: banner names the layer (VIM NORMAL / VIM INSERT / …), keys and combos light, one-shot Alpha 2 shown, ✕ closes |
| `hud/keys.html` (typed-keys strip) | works, replaces KeyCastr |
| `hud/hud.lua` (macOS host, Hammerspoon) | works: event tap, daemon-log polling, two overlay windows, close button via user-content controller |
| `hud/rehearse.lua` + `rehearse.sh` (macOS rehearsal) | segments 3–8 all PASS individually; `all` not yet re-run after the last three changes (no Esc after opening IntelliJ's file, segment 8 via the demo shell, editor resets moved to `prepare.sh`) |
| `prepare.sh` (macOS) | written after the last rehearsal; **not yet run by the user** |
| `setup.sh` | now a git-based content reset (content is committed); works |
| `linux/*` | **written, never run** — see below |

Open issues the user reported that are not resolved:

1. **IntelliJ shows an "IDE error" popup** at some point during the rehearsal. The user did not
   send the exception yet. Check `idea.log` (Help → Show Log) for the last `ERROR`; if the trace
   mentions `dev.rafaelromao.zmkvimmode`, it is our plugin (`editors/intellij`), otherwise it is
   IdeaVim/platform noise from synthesized keys.
2. **IdeaVim's `:` reports `raw`, not `cmdline`.** `editors/intellij/README.md` promises cmdline.
   The ex-entry field is a separate component, so the plugin's focus listener wins over the mode
   change. Either fix the plugin (treat the ex-entry focus as cmdline) or fix the README. The
   script currently documents the observed behaviour.
3. The user has not confirmed the ✕ button, the top-right HUD position without the old
   backdrop, or the strip-band window placement in `prepare.sh` — all done in the last hour.

## Layout of `showcase/`

```
SCRIPT.md               the video script (beats, narration, actions, expected reasons, shot list)
README.md               how to run the kit (macOS commands)
HANDOFF.md              this file
keymap/build.py         drawer YAML (keyboards repo) → hud/keymap.json + keymap.js (gitignored)
hud/                    HUD pages (portable) + macOS host
  index.html hud.js hud.css     layer HUD: renderer + keymap emulator; window.hud API
  keys.html                     typed-keys strip; window.keys API
  hud.lua place.lua rehearse.lua start.sh   Hammerspoon host, window placement, rehearsal runner
linux/                  Omarchy host (UNTESTED): keyfeed.py hud.sh prepare.sh rehearse.py
Demo/                   Obsidian vault (notes, attachments, .obsidian settings; runtime state ignored)
demo-go/                Go module for Neovim and VS Code (modes table); demo-go.code-workspace
demo-java/              plain Java project for IntelliJ (.idea/ ignored)
env/                    ghostty-demo.conf, .zshrc (demo shell), obs-scene.md, privacy-checklist.md
setup.sh                content reset (git checkout of Demo, demo-go, demo-java) + GUI steps
prepare.sh              macOS: quit/clean/reopen/place the editors
rehearse.sh             macOS: run a segment through Hammerspoon
run/                    gitignored runtime: rehearsal.log, demo shell history, keyfeed log
```

## How the HUD works (both platforms)

The pages are host-agnostic. They expose:

- `hud.load(data)`, `hud.setMode(code, mode, reason)`, `hud.key(event)`, `hud.press([idx])`
- `keys.key(event)`

An event is `{type: keyDown|keyUp|flagsChanged, name, chars, code, flags:{cmd,ctrl,alt,shift,fn}, repeat}`
with `name` spelled like Hammerspoon's `hs.keycodes.map` (`space`, `return`, `escape`, `delete`,
`tab`, `left`…). Two host modes:

- **Hammerspoon** evaluates JavaScript into the WKWebView (`hud.lua`).
- **WebSocket**: open a page as `index.html?ws=ws://127.0.0.1:8766`; it connects and accepts
  `{"kind":"key",…}` and `{"kind":"mode",code,mode,reason}`; the ✕ button sends `{"kind":"close"}`.
  `linux/keyfeed.py` speaks this.

Emulation rules (in `hud.js`, agreed with the user):

- Vim layers come from the daemon (ground truth). Codes 1/3/4/7 show the drawer's `vim` layer
  (banner *VIM NORMAL* / *VIM VISUAL*); 2 and 5 show *VIM INSERT* / *VIM CMDLINE* with Alpha 1
  legends (those layers are transparent on the keyboard); 6 *RAW*, 0 *ALPHA 1*. Local inference
  for `i a o s c`, `Esc`, `v`, `:` `/`, `Enter` moves the banner instantly (dashed underline
  until the daemon confirms).
- Typing goes through the two alpha layers: a letter not on Alpha 1 is the sticky Alpha 2
  (one shot, shown for 450 ms). The Alpha 1 letter combos (`ns=q`, `mg=k`, …) are **vim
  commands only** and are considered only while the vim layer is active.
- A key found on another layer means a held (`numbers`, `symbols`, `nav`, `shortcuts`) or
  one-shot layer the host could not see; its activator thumb lights; dropped after the next
  base-layer key or 700 ms.
- Combos draw a pill (output + meaning) linked to their keys, like keymap-drawer's dendrons.
- Auto-repeats are ignored; the strip merges plain characters into one chip.

Data: `keymap.json` — `layout.keys[i]` (hand/row/col/ZMK position), `layers[name][i]`
(`tap/hold/shifted/type`), `combos[]` (`positions/key/layers`), `activators[]`, `codes`.
Drawer index ↔ ZMK position: rows 0–2 = ZMK 1 2 3 | 6 7 8, 10–13 | 16–19, 21 22 23 | 26 27 28;
thumbs 20 21 | 22 23 = ZMK 31 32 | 33 34.

## What differs on Omarchy

The daemon side is already Linux-first (that was the v1 target): systemd user unit, hidraw,
udev rule `contrib/udev/60-zmk-vim-mode.rules`, Hyprland focus backend, AT-SPI2 for VS Code's
widgets. The HUD host and the automation are what change:

| Concern | macOS | Omarchy |
|---|---|---|
| Key events | Hammerspoon event tap | `linux/keyfeed.py`: evdev on every keyboard (needs read access: the ZMK device has uaccess from the udev rule; other keyboards need the `input` group) |
| Daemon decisions | `~/Library/Logs/zmk-vim-mode.log` (launchd stderr), size polled | `journalctl --user -u zmk-vim-mode -f -o cat`, parsed for `msg=decision`; fallback: poll `status --json` |
| HUD windows | `hs.webview` overlays | Chromium `--app=file://…?ws=…` with `--class=zmkhud`; Hyprland `windowrulev2` float/pin/nofocus/move/size; bottom band reserved with `monitor addreserved` (`linux/hud.sh`) |
| Typing in rehearsals | `hs.eventtap.keyStrokes` | `wtype` (Wayland); chords via `-M/-m` |
| Focus / placement | `hs.window`, `hs.application` | `hyprctl dispatch focuswindow class:^(…)$`, tiling + reserved band |
| Mouse click (Obsidian back-to-note) | `hs.eventtap.leftClick` | `hyprctl dispatch movecursor` + `ydotool click 0xC0` or `wlrctl pointer click left` |
| Editor resets | `prepare.sh` (osascript quit, `open -a`) | `linux/prepare.sh` (`pkill`, `code`, `idea`, `xdg-open obsidian://…`) |
| Shortcuts in segments | ⌘-chords | Ctrl-chords (Ctrl+P, Ctrl+Shift+E, Ctrl+`, Ctrl+Shift+N, Alt+F12, Ctrl+E, Ctrl+Shift+F); Meh+B identical |
| Ghostty demo window | `open -na Ghostty --args --config-file=…` | `ghostty --config-file=… --class=zmk-showcase`; macOS-only keys in `env/ghostty-demo.conf` (`macos-titlebar-style`) are ignored with a warning |
| Display | 27" HiDPI 1080p via displayplacer | Hyprland monitor config; OBS Display Capture (PipeWire) |

Things to verify first on the box, in this order:

1. `zmk-vim-mode status` / `doctor` green; the Diamond `writable` over USB or BLE.
2. `python3 showcase/keymap/build.py` (needs `yq` v4 and the keyboards repo at
   `~/projects/keyboards`; set `KEYBOARDS_REPO` otherwise).
3. `bash showcase/linux/hud.sh`, then `zmk-vim-mode set normal` → the banner must flip to
   *VIM NORMAL*; type on the Diamond → keys light on the right layer. Check `run/keyfeed.log`
   if not. Likely first problems: evdev permissions; Chromium not honouring `--class`
   (then match `title:` in the window rules); the `?ws=` query being stripped from a `file://`
   URL by the browser (then serve `hud/` with `python3 -m http.server` and use `http://`).
4. `bash showcase/linux/prepare.sh`, then `python3 showcase/linux/rehearse.py 3`, then `4`
   with `--verbose` and adjust timings; then the rest. The Lua runner's step list is the
   reference; the Python port is 1:1.
5. Obsidian on Linux must be the native package, not Flatpak (the plugin cannot reach the
   socket from Flatpak); the vault to register is `showcase/Demo` (folder name = vault name
   `Demo`, used in the `obsidian://` URIs).

## Lessons that cost time (do not repeat)

- Hammerspoon: `hs.json.encode` takes only tables (strings must be quoted by hand); unreferenced
  `hs.timer` objects are garbage-collected mid-chain; `keyStrokes` paces characters but a
  separate `keyStroke(return)` overtakes them; an empty Lua table encodes as `[]` and
  JavaScript arrays have a `.shift` method (that is why the strip showed ⇧ on every key).
- Java apps ignored a synthesized `⌘1`; use the Mehs keymap's chords (`Meh+B` project window,
  `Hyper+U` find action). `⇧⌘A` is remapped by Karabiner on the Mac.
- IdeaVim rings the bell on `Esc` in normal mode: never send a redundant `Esc` there
  (`set visualbell` in `~/.ideavimrc` silences it for the recording).
- In VS Code's Explorer a letter creates a file in this user's setup; use arrows there.
- LazyVim: leader-pending `raw` lasts only `timeoutlen`+50 ms (~350 ms); the picker input is
  insert mode; leave `:terminal` with `Ctrl-\ Ctrl-n`, two Escapes race the mapping.
- Enter/Tab are combos on almost every layer (`cb_enter` RHM RHR R0, `cb_tab` RTM RTR R0) even
  though the drawer YAML lists them only under `shortcuts`; `build.py` patches the layers.
- Everything typed in a tool window should start with a letter that is a home-row motion on the
  vim layer (`readme`, `keyboard`, `layer`, `ls`) so the RAW state reads on camera.
- Editors keep state between runs (dirty files, panels, reading view, tool windows) that changes
  what the daemon reports: reset once with `prepare.sh` rather than sending close commands
  mid-run (those caused their own beeps and errors).
- Claude Code's sandbox on the Mac could not reach the daemon socket, the Hammerspoon Mach
  port, network sockets, nested `.git` dirs or dotfiles; the user ran the scripts and pasted
  output, the agent read `run/rehearsal.log` and the daemon log. Expect similar limits.

## Suggested next steps

1. Run `prepare.sh` + `rehearse.sh all` once more on the Mac, or the Linux equivalents on
   Omarchy; fix whatever the last three changes broke.
2. Settle the two IntelliJ items above (error popup, cmdline vs raw).
3. Record beats 3–8 one take each following `SCRIPT.md`; beats 0–2 and 9 are title cards,
   diagrams from `~/projects/keyboards/docs/img/diagrams/` and a pre-rendered terminal.
4. Optional polish the user mentioned wanting: nothing pending beyond the above.

## Sources of truth

- Keymap: `~/projects/keyboards` (`docs/img/diagrams/keymap-drawer/keymap-drawer.yaml`,
  `src/definitions/config.dtsi`, `src/features/vim.dtsi`, `src/features/combos.dtsi`).
- Daemon protocol and reasons: `internal/proto/proto.go`, `internal/state/`, the editor READMEs
  under `editors/`.
- The plan this was executed from: the "Context" and "Benefits analysis" sections are
  reproduced in `SCRIPT.md`'s argument paragraph and beats 2–3.
