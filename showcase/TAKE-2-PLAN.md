# Showcase video, take 2: what the first take showed and what changes

Written 2026-09-19 from the first assembly, `showcase-takes.mp4` (6:10, 2560×1440 @ 30 fps):
`record.py`'s automated takes 3–8 of the first script, joined by concat, with the piper TTS
scratch narration (22.05 kHz mono is piper's native rate). It proved the pipeline end to end:
the daemon, the four editor plugins and the zmk-layer-hud panel all behaved on camera, hands-off.
It was not yet a YouTube video. This document is the review and the production plan; the script
itself is `SCRIPT.md`, the packaging is `YOUTUBE.md`.

**Format decision:** the video is screen recording plus narration, nothing else — no camera on
the keyboard, no animation, no callouts, no music. The HUD and the mode line are the graphics.

---

## 1 · What the first take shows

### Timeline

| Time | Screen | Audio |
|---|---|---|
| 0:00–0:47 | wallpaper → demo terminal: `status`, `journalctl` pane, `set insert/normal/off` | narrated |
| 0:47–1:16 | Neovim being opened (dashboard) | **silence 29 s** |
| 1:16–2:10 | Neovim tour, `/Compose`, visual, `:s`, explorer, `:terminal` | narrated |
| 2:10–2:43 | leaving Neovim, opening VS Code | **silence 33 s** |
| 2:43–3:23 | VS Code tour, terminal, palette, explorer | narrated |
| 3:23–4:00 | opening IntelliJ, `Ctrl+Shift+N` | **silence 36 s** |
| 4:00–4:39 | IntelliJ tour, project tree, terminal | narrated |
| 4:39–4:59 | opening Obsidian | **silence 20 s** |
| 4:59–5:31 | Obsidian tasks, reading view, search | narrated |
| 5:31–5:47 | back to the shell | **silence 16 s** |
| 5:47–6:06 | `status`, `set raw`, `exit` | narrated |
| 6:06–6:10 | wallpaper; HUD panel empty, reading *waiting for the keymap…* | silence |

The clock in the top bar reads 11:40 in Neovim, 11:42 in VS Code, 11:43 in IntelliJ, 11:44 in
Obsidian and 11:57 in the terminal (take 3 was re-recorded last): the bar gives the concat away.

### Findings, by impact on YouTube performance

1. **No hook.** First 10 s: wallpaper, then a wall of `zmk-vim-mode status` output. Both
   audiences decide in that window. The strongest thing the project has — the board changing on
   `Esc` — appears at 1:16, after a minute of daemon internals.
2. **Legibility.** At 1440p the HUD board is ~350 px wide (14 % of the frame), key legends are
   ~7 px tall, editor code ~14 px. YouTube serves most views at 1080p or on a phone. The notes
   already asked for ≥ 18 pt text and a fractional Hyprland scale; neither was applied.
3. **Dead air: 137 s of 370 s.** The segments type every state the rehearsal checks, so each
   take runs 20–36 s longer than its narration, and the TTS stops while the picture goes on.
   The narration is now shorter still, and each automated beat carries a *Cut* line.
4. **Repetition.** Four editors × the same `hjkl / web / i / Esc` tour is ~3 minutes of the same
   picture. Neovim earns the full tour; the others earn one distinctive beat each.
5. **Audio.** The TTS scratch, by design: mono, 22.05 kHz, peaks at 0 dB. Not a defect of the
   recording, but the deliverable needs the human read, cut first and dubbed after.
6. **Take errors left in, all from the driver:** `bash: led: command not found` at 0:30 —
   `seg3` typed `grep -E 'decision|led'` and ydotool dropped the quotes (now `grep -e decision
   -e led`, no quotes to lose); `// bit 1 of the code` typed twice in VS Code (the `/Kana` search
   landed on a line that already carried the comment from an earlier run — `prepare.sh` before
   every run); the note text ` (rehearsal)` typed by `seg7` (now ` publish the video`); **Tasks**
   shown twice (Obsidian's inline title plus an `# Tasks` H1, removed); the assembly ends on an
   empty HUD, which is the rehearsal's clean-up tail after the last take (cut it).
7. **Desktop clutter.** Waybar with workspaces, CPU temperature, battery, weather and a changing
   clock. Its one useful module, the daemon mode (`NORMAL / INSERT / RAW`), was 11 px in a corner.
8. **Empty frame.** Terminal beats hold one prompt line and 90 % dark screen for 20 s.
9. **HUD timing.** `press_ms` is 320 ms and `record.py` defaults to 30 fps: a tap is ~9 frames,
   borderline after YouTube's compression. `ZMK_RECORD_FPS=60`, `press_ms` 500.
11. **Beat 8 showed nothing.** `seg8` ran `set raw` through `subprocess`, not through the
    keyboard, so the terminal stayed empty while the narration described the escape hatch. It
    now types the commands.
10. **The physical keyboard never appears.** Accepted: the video stays screen-only. The narration
    therefore names the board in the cold open ("that's my keyboard, drawn live from its own
    reports") and `Diamond.jpeg` appears once, in the image viewer.

What already works and stays: the HUD banner following the editor exactly (*Vim normal*, *Vim
insert*, *Vim visual · Vim normal*, *Alpha 1*), the typed-keys strip, the demo content
(`modes.go`, `ModeTable.java`, the Demo vault — on topic, no private data), and `status` as the
judge of a take.

---

## 2 · Production changes for take 2

Detailed in `env/obs-scene.md`; the decisions:

### Picture

- **Logical 1080p on the 1440p monitor**: `hyprctl keyword monitor <NAME>,2560x1440,auto,1.333333`
  for the session (`hyprctl reload` reverts). Everything, HUD included, grows by a third; the
  598 pt panel becomes ~31 % of the frame. Record native, output 1920×1080 at **60 fps**.
- **HUD for the take**: `hud.press_ms` 320 → 500 in `~/.config/zmk-layer-hud/config.yaml`.
- **Every beat automated**: `ZMK_RECORD_FPS=60 python3 showcase/record.py all`, after scaling,
  hiding waybar and placing the mode line; the take HUD stopped first. New segments 0, 1, 2 and 9
  print text cards (`env/cards/`) in the demo shell and open the keyboards repo's images in `imv`.
- **Waybar hidden** (`pkill -SIGUSR1 waybar` toggles it). Start the HUD after hiding it.
- **Mode line**: a pinned Ghostty at ~28 pt in the empty part of the rail running
  `watch -n 0.2 -t 'zmk-vim-mode status | head -1'`. It puts the daemon's reason on camera and is
  what tells RAW from OFF, which the HUD cannot.
- **Non-demo shots are screen recordings, driven by the same segments**: text cards printed in
  the demo terminal (title, the descriptor bytes and code table, the pipeline diagram, the
  devicetree node, the links), the layer PNGs and `Diamond.jpeg` maximized in the image viewer,
  and the real `doctor` with home paths masked by a shell function.
- **Fonts** as `obs-scene.md` prescribes; verify in the first frame of each clip.

### Sound

- Record every clip **silently**, one per beat.
- Cut the picture first (every silence out, plain cuts), then read `SCRIPT.md` against the cut on
  a USB or headset mic at 48 kHz, normalized to −16 LUFS, no clipping. Reading live is acceptable
  if each beat is its own clip and the mic is not the phone.
- Trim picture to words, not words to picture.

### Edit and delivery

Plain cuts, no zooms or callouts, an 8 s end card, no silent tail. 1920×1080, 60 fps, stereo
48 kHz, no silence over 1.5 s. Move the master over Drive or AirDrop, not WhatsApp.

### Pre-flight, every clip

`prepare.sh` run · monitor scaled · waybar hidden · HUD drawing the board (never *waiting for the
keymap…*) · typed-keys strip empty, no `(rehearsal)` chip · mode line showing the beat's expected
reason · no error line in any terminal on screen · `git status` clean in the demo dirs.

### Content fixes made in the repo

- `Demo/Tasks.md`: the `# Tasks` H1 removed (Obsidian shows the filename as title).
- `rehearse.py`, three string edits, **not yet run on the box**: `seg3`'s grep without quotes,
  `seg7` types ` publish the video` instead of ` (rehearsal)`, `seg8` types `zmk-vim-mode set raw`
  instead of running it out of band. Re-run `python3 showcase/rehearse.py 3`, `7`, `8` before
  recording.
- `env/cards/`: the text cards, including the four URLs for the end card.

---

## 3 · The script

`SCRIPT.md`: ten beats, 694 words, ~5:00. Every beat is a segment of `rehearse.py` with the same
number, so `record.py all` records the whole video hands-off; the tables show what the segments
type and a *Cut* line says what to keep.

| Beat | What | How | Time |
|---|---|---|---|
| 0 | Cold open: `Esc` in Neovim, the HUD flips, `hjkl` on the home row | `record.py 0` (new `seg0`) | 0:18 |
| 1 | Title card, `Diamond.jpeg` | `record.py 1` (new `seg1`, silent) | 0:07 |
| 2 | The problem: `alphas.png` (Alpha 1 + Alpha 2) vs `vim.png` | `record.py 2` (`seg2`) | 0:40 |
| 3 | How it works: channel and pipeline cards, then `set` in two panes | `record.py 3` (cards first) | 0:50 |
| 4 | Neovim, the one full tour, including *raw* | `record.py 4` | 1:00 |
| 5 | VS Code: terminal and palette | `record.py 5` | 0:20 |
| 6 | IntelliJ: project tree and terminal | `record.py 6` | 0:20 |
| 7 | Obsidian: reading view and search | `record.py 7` | 0:23 |
| 8 | Everywhere else, the escape hatch, USB + Bluetooth | `record.py 8` | 0:25 |
| 9 | `doctor` masked, the devicetree node, the links | `record.py 9` (new `seg9`) | 0:35 |

Changes from the first script: hook before explanation; one full tour instead of four, the
other editors at ~40 words each; the pass/fail judge is the mode line on screen; the "how it
works" material is the README itself, on screen, instead of separately produced cards.

## 4 · Packaging

`YOUTUBE.md`: three title candidates, thumbnail (the Diamond photo plus a HUD frame, two words),
description with chapters, tags, pinned comment, posting order (ZMK Discord, r/ErgoMechKeyboards,
r/neovim, r/vim, Show HN, keymap-drawer discussions), an 18 s vertical Short cut from the cold
open, and zmk-layer-hud alone as the second video.

## 5 · Files touched for take 2

| File | Change |
|---|---|
| `SCRIPT.md` | second script, beats 3–8 aligned with `rehearse.py`'s segments |
| `YOUTUBE.md` | new: packaging |
| `TAKE-2-PLAN.md` | this document |
| `env/obs-scene.md` | scale, 60 fps, waybar, mode line, silent takes, voice after cut, delivery |
| `env/privacy-checklist.md` | take-hygiene block |
| `Demo/Tasks.md` | duplicate heading removed |
| `rehearse.py` | new `seg0`, `seg1`, `seg2`, `seg9` and `view_image()`; cards before `seg3`'s live part; three typed strings: `seg3` grep, `seg7` note text, `seg8` typed `set raw` — **none run on the box yet** |
| `record.py` | beats 0–9; a beat without narration records silently |
| `env/demo.bashrc` | `show <card>` and `doctor` (home paths masked) |
| `env/cards/` | title, channel, pipeline, node, links |
| `README.md`, `HANDOFF.md` | pointers and the dated entry |

The segments still type the first script's full tours in beats 5–7; they exercise every state the
rehearsal checks and the *Cut* lines trim them in the edit. Trimming the segments themselves is a
later choice, once the takes are in hand.
