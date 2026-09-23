# Handoff — zmk-vim-mode showcase video, take 2 (recording session, in progress)

Started 2026-09-19 ~14:45 -03. Task: follow `showcase/TAKE-2-PLAN.md` and record the video now.
The plan, script, scene spec and packaging live in the repo — read them, do not re-derive:

- `/home/romao/projects/zmk-vim-mode/showcase/TAKE-2-PLAN.md` — the production plan (follow this)
- `showcase/SCRIPT.md` — ten beats, what each segment types, Cut lines, pre-flight
- `showcase/env/obs-scene.md` — scale, 60 fps, mode line, silent takes, delivery
- `showcase/record.py`, `showcase/rehearse.py`, `showcase/prepare.sh`, `showcase/hud.sh`
- `showcase/HANDOFF.md` — earlier sessions' context

## State when this session ended

Done:

1. Repo fixes already committed (HEAD `98be283`, git clean): `Demo/Tasks.md` H1 removed;
   `rehearse.py` carries the three string edits (seg3 grep without quotes ~line 416, seg7 types
   ` publish the video` ~line 537, seg8 types `zmk-vim-mode set raw` ~line 563); `env/cards/`
   exists (title, channel, pipeline, node, links).
2. First-assembly takes archived to `showcase/run/First assembly/` (take3–8 `.mp4`/`.wav`) so
   `record.py`'s overwrite loses nothing; `run/showcase-takes.mp4` left in place.
3. Take HUD stopped (`bash showcase/hud.sh stop` → "panel reservation released") and the stray
   rehearsal HUD killed; verified no HUD processes running.
4. piper-tts installed: `~/.cache/zmk-showcase/tts/bin/piper` (voices were already at
   `~/.cache/zmk-showcase/voices/`; `en_US-ryan-medium.onnx` is `record.py`'s default).

Box state verified: single monitor **HDMI-A-1** 2560×1440 (scale was 1.25, needs 1.333333), no
laptop attached; waybar/omarchy-bar **not running** (stale `omarchy-bar` layer has `geometry:
null` — nothing to hide); ydotoold running; `zmk-vim-mode status` shows no override; the HUD
venv (`~/projects/zmk-layer-hud/.venv`) is healthy; keyboards-repo images exist
(`docs/img/builds/Diamond.jpeg`, `docs/img/diagrams/{alpha1,vim}.png`).

## Findings not in the plan docs

- **press_ms flow**: `rehearsal-feed.py` → `hudfeed.Feed(config=…)` whose default is
  `~/.config/zmk-layer-hud/config.yaml` (`keymap_mod.DEFAULT_CONFIG`) → websocket → `hud.js`
  `T('press_ms')`. Editing that config file applies to the rehearsal HUD (hardcoded 598×392
  panel in `rehearsal-panel.py`); picked up because the HUD restarts per segment.
- **`rehearsal-panel.py` geometry**: HUD anchored top-right, margin top = min tiled top + 8,
  right = width − max tiled right edge + 8; keys strip 598×96 below it (top + 392 + 8); rail
  reservation = 598 + right + 8 (exclusive, TOP+BOTTOM+RIGHT) — windows maximized *after* it
  starts shrink to exclude it. Compute the mode line's position from this after `prepare.sh`.
- **Mode line**: `zmk-vim-mode status | head -1` prints e.g.
  `decision : off (code 0) — terminal without nvim client` (52 chars; longest on-camera reason
  ~66–70 chars, `tool window focused: widget outside any view`). At ~28 pt in a 598-wide
  window it wraps; chosen approach: window ~760 logical px wide, `--font-size=24`, verify with
  a frame grab and adjust. Give it a **distinct class** (`zmk-modeline`) so it can never
  interfere with `DEMO_CLASS` matching; float + `hyprctl dispatch pin`; spawn with `nohup … &`
  (it blocks otherwise).
- **`record.py` mux**: video `tpad` stop 5 s + `apad` + `-shortest` → TTS takes = raw + 5 s;
  beat 1 (no narration) is renamed raw, no tail.
- **pkill self-match gotcha**: `pkill -f "showcase/rehearsal-panel.py"` matches the agent's own
  bash cmdline and kills the shell (caused a tool timeout). Use `rehearsal[-]panel.py`.
- The bash tool has `~/.local/bin` on PATH (`zmk-vim-mode` works directly).
- `rehearse.py` starts its rehearsal HUD *before* segments; `record.py` invokes `rehearse.py`
  per segment, so each beat brings its own HUD (that is why the take HUD must be stopped first).

## Remaining steps (continue here)

1. `press_ms: 320` → `500` in `~/.config/zmk-layer-hud/config.yaml` (line 16). Revert after.
2. Scale monitor: `hyprctl keyword monitor HDMI-A-1,2560x1440,auto,1.333333` (logical 1080p).
3. `bash showcase/prepare.sh` — few minutes (IntelliJ relaunch with 25 s settle + indexing).
4. Verify edited segments per the plan: `python3 showcase/rehearse.py 3`, then `7`, then `8`;
   also run `1` (view_image/`show title` is brand new, used by beats 1, 2, 9). Read
   `run/rehearsal.log` results before the recording overwrites them.
5. Place the mode line (numbers above), then a frame grab (`grim`, or ffmpeg on a short
   gpu-screen-recorder capture) to check font/position/wrap; adjust.
6. Pre-flight (SCRIPT.md *Pre-flight, every run*): git clean in demo dirs, no override, HUD
   drawing the board (never *waiting for the keymap…*), no error lines, strip empty.
7. Record: `ZMK_RECORD_FPS=60 ZMK_TTS_BIN=~/.cache/zmk-showcase/tts/bin/piper python3
   showcase/record.py all`. Decision made: **with TTS scratch** (the plan's verbatim command;
   record.py's docstring and SCRIPT.md both say takes carry the scratch). ~20–25 min; hands off
   — no window/input activity on the box while it runs; run via a PTY with `notifyOnExit` and a
   generous timeout. Takes land in `run/take<N>.mp4`.
8. Verify takes: `run/rehearsal.log` PASS/FAIL per beat, ffprobe durations, extract frames with
   ffmpeg and view them (read tool renders images) — HUD board, mode line, fonts, legibility,
   no error lines.
9. Revert session: `hyprctl reload` (scale back to 1.25), `press_ms` back to 320, kill strays.
   If takes failed, consider keeping the setup and asking the user about retaking first.

After the picture exists, the deliverable still needs the human read (`SCRIPT.md` against the
cut, mic at 48 kHz, −16 LUFS) and the packaging in `showcase/YOUTUBE.md` — the user's job, or a
later session's.

## Session 2026-09-20 — corrections before the re-record (in progress)

User corrections to the plan above, all applied:

- **Scale is 1.25, not 1.333.** `hyprctl keyword monitor …` does not work on this box
  (non-legacy parser); set it through Lua instead:
  `hyprctl eval 'return hl.monitor({output="HDMI-A-1", mode="2560x1440@59.95",
  position="0x0", scale=1.25})'`. Note: asking for 1.5 snaps to 1.6 (logical 1600×900) —
  1.5 gives fractional pixels on 2560 wide. `SCRIPT.md` and `env/obs-scene.md` now say 1.25.
- **Beat 3 uses one Ghostty window split down, not tabs or two tiled windows** (`rehearse.py`
  `seg3`: `ctrl+shift+e`, the lower pane tails the log, and `ctrl+alt+up/down` switches focus).
  The generated `run/ghostty.conf` gives every split the demo Bash prompt. `SCRIPT.md` beat 3
  and shot D are updated.
- **Menu-bar widgets stay as configured.** An earlier take stripped `romao.agents` and
  `romao.weather`, but the user reported missing icons. `prepare.sh` now leaves
  `~/.config/omarchy/shell.json` untouched and must not remove or disable any bar item or restart
  QuickShell.
- `press_ms` is at 500 (revert to 320 after). Demo terminal font stays 20.

State: `prepare.sh` re-ran at 1.25 but failed on IntelliJ showing no window within a
minute (process up, `run/idea.log` empty) — retry in progress. `seg7` was interrupted
mid-run by the scale correction; its rehearsal HUD was swept. Re-verify 3, 7, 8, 1
after prepare goes green, then continue at *Remaining steps* 5.

## Session 2026-09-20 (cont.) — recording results

All ten takes recorded green at 1.25x, 2560×1440 @ 60 fps (`run/take0–9.mp4`):
0: 4/4, 1: 1/1, 2: 1/1, 3: 4/4 (tabbed), 4: 27/27, 5: 20/20, 6: 19/19, 7: 15/15
(retake), 8: 4/4, 9: 1/1. Mid-take frames verified for 0, 1, 2, 3, 5, 6, 7, 9:
banner, lit keys, strip and mode line all on camera; the Diamond photo shows in
take 1 (imv cold start is slow — the photo lands ~20 s in, keep the tail of the
raw); take 9's doctor reads `0 failed, 0 warnings, 16 checks` with `~` paths.

- **Take-7 stall, root-caused.** Obsidian died mid-run (the known transient); the fresh
  `xdg-open` never returned, `rehearse` sat in `do_wait` 34 min and the driver (which has
  no per-segment timeout) stalled with it. Freed by killing the segment (driver kept the
  raw and moved on, as designed), relaunched Obsidian with prepare's
  `--in-process-gpu` flags, retook beat 7 alone → 15/15. `sh()` in `rehearse.py` now has
  `timeout=120` so a stuck open fails loud instead. If Obsidian is ever windowless,
  `xdg-open` hangs — check `hyprctl clients` before blaming the segment.
- **Mode line hides on beats 1, 2, 9 now.** The pinned box reading "off — terminal
  without nvim client" over cards/photos is noise, so `record.py` stops it for those
  beats (`showcase/modeline.sh stop`) and restarts after. `modeline.sh start` is also the
  codified placement (title `zmk-modeline`, float, both fullscreen modes unset, 617 wide
  at x1431 y558, pinned; font 16, no decorations). Takes 1, 2, 9 still need retaking
  with the box out of frame.
- Lua window calls that work here: `float({window=…, action='toggle'})`,
  `fullscreen({mode='maximized'|'fullscreen', action='unset'})`, `resize({window=…,
  x=…, y=…})`, `move({window=…, x=…, y=…})`, `pin({window=…})` — via `hyprctl eval
  "hl.dispatch(…)"`. `hyprctl dispatch <classic>` does not parse (Lua router).
  Ghostty `--class=` with a custom value does not stick; `--title=` does. A resize
  races an in-flight un-maximize: size first, then move twice, then pin.
- `prepare.sh` also clears VS Code demo hot-exit backups now (a restored dirty
  `modes.go` buffer put seg5's old comment back on camera with a clean disk).

## Session 2026-09-20 (cont.) — audio refit, name out (done)

Scratch was ~165wpm from take start (~9 s before the first keystroke), so every beat
crammed its words into the first third. Re-rendered all ten at length-scale 1.77
(~140wpm, the script's pace; `record.py` default updated from 1.3) with the name out
of beat 1 (48→45 words; beat times −0:02 from beat 2 on, total 739 words 5:10,
`YOUTUBE.md` chapters shifted), re-muxed onto the untouched video with a 9 s head
delay (`adelay=9000,apad`, `-c:v copy`, take durations unchanged), assembly rebuilt
(10:08). Verified: digital silence 0–8 s, speech after. Uncommitted: SCRIPT, YOUTUBE,
record.py.

## Session 2026-09-20 (cont.) — full restart, all green (done)

Restarted from prepare at 1.25x with the no-modeline/intro/ws8-parking script: all ten
takes green in one run, exit 0 (0: 4/4, 1: 2/2, 2: 1/1, 3: 5/5, 4: 27/27, 5: 20/20,
6: 19/19, 7: 15/15 with no stall, 8: 4/4, 9: 1/1). Reverted: `press_ms` 320, bar
widgets restored and frame-verified, strays swept. Uncommitted: SCRIPT, YOUTUBE,
obs-scene, record, rehearse, `env/cards/intro.txt`. Takes in `run/` (this run).

- Beat 1 is the intro now: `show title` 4 s, `show intro` (`env/cards/intro.txt`) 14 s,
  Diamond photo 3 s, with a 48-word narration (beat times shift +0:14 from beat 2 on;
  total 742 words, 5:12; `YOUTUBE.md` chapters updated).
- Beats 3 and 8 carry the reason in typed `status` lines (closing `status` in seg3,
  `status | head -1` around each `set raw` in seg8) — no pinned panel in any take.
  `record.py` stops the mode line on every take and parks on workspace 8 first, so take
  heads show wallpaper/demo content, never workspaces 1–4. Take tables now say
  "daemon reason"; `modeline.sh` stays for manual debugging only.
- Take 7 failed the same windowless-Obsidian way (this time to the new `sh()` timeout,
  loud after 120 s). seg7 no longer waits on `xdg-open` at all: fire-and-forget Popen
  plus the existing `wait_window` gate. Relaunched healthy, retook 15/15.
- The earlier quickshell crash note above still holds: if that dialog appears on a
  bar restart, confirm the supervisor's relaunch and move on.

## Suggested skills

- `implement` — if continuing the remaining steps as plan-driven work item execution.
- `diagnosing-bugs` — if a segment/take fails mid-run and the cause isn't obvious (start with
  `run/rehearsal.log`, `run/record-gsr.log`, `run/rehearsal-panel.log`, `run/rehearsal-feed.log`).
- `remember` — save the pkill self-match gotcha, the press_ms flow and the mode-line geometry
  findings for future sessions on this project.
