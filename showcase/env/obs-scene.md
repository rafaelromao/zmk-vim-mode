# OBS scene for the showcase

Recorded on Omarchy Quattro (Hyprland). Fill the display table in on the box itself:

```bash
hyprctl monitors -j | jq -r '.[] | "\(.name)\t\(.width)x\(.height)@\(.refreshRate|floor)\tscale \(.scale)\tfocused=\(.focused)"'
```

| Monitor | Mode | Role |
|---|---|---|
| `eDP-…` (laptop) | | OBS, notes, this script |
| the external one | | **recording monitor** |

Both stay on: the recording monitor holds only the editor and the HUD; OBS and the script live
on the laptop. Nothing picks the monitor by hand — `hud.sh`, `prepare.sh` and the rehearsal all
use the first monitor whose name does not start with `eDP`.

## Canvas and output

**Set the recording monitor to scale 1.25 for the session.** A fractional Hyprland scale
enlarges every app and the HUD at once (on 2560×1440 that is a logical 2048×1152); the
first take (2026-09-19) recorded at native 1440p and
the HUD's key legends were ~7 px tall:

```bash
hyprctl eval 'return hl.monitor({output="<NAME>", mode="2560x1440@59.95", position="0x0", scale=1.25})'
```

`hyprctl reload` puts your config back afterwards. Start the HUD *after* setting the scale and
hiding waybar, so it reserves the rail against the final geometry.

| Setting | Value |
|---|---|
| Settings → Video → Base (canvas) | the recording monitor's mode (2560×1440) |
| Settings → Video → Output (scaled) | 1920×1080, Lanczos |
| FPS | **60** (the first take was 30: a 320 ms key flash is ~9 frames, too few after YouTube's compression) |
| Settings → Output → Recording | MKV then remux to MP4, H.264 (VAAPI/NVENC) CQ ~18 |
| Audio | **none during takes** — the voice-over is recorded afterwards (see *Sound*) |
| Text sizes | the table below is mandatory |

For the take, raise `hud.press_ms` from 320 to 500 in `~/.config/zmk-layer-hud/config.yaml`
(`hud.combo_pill_ms` 1000 is fine) and restart the HUD. Revert after recording.

## Sources

1. **Screen Capture (PipeWire)** — pick the recording monitor in the portal dialog, cursor
   shown. Wayland has no "capture that window forever" without the portal, so re-pick it if the
   session restarts.
2. Nothing else. The HUD's surfaces are real layer-shell windows on that monitor, so they are
   captured as-is and you see exactly what the viewer sees while presenting.

## Window layout on the recording monitor

```
┌──────────────────────────────────────────────────────────────┐
│                                        ┆┌────────────────┐   │
│                                        ┆│ layer HUD      │   │
│   editor / terminal,                   ┆│ 598×392        │   │
│   maximized (never fullscreen)         ┆└────────────────┘   │
│                                        ┆┌────────────────┐   │
│                                        ┆│ typed keys  96 │   │
│                                        ┆└────────────────┘   │
│                                        ┆┌────────────────┐   │
│                                        ┆│ mode line      │   │
│                                        ┆└────────────────┘   │
│                                        ┆ reserved rail ~617  │
└──────────────────────────────────────────────────────────────┘
```

- **HUD**: `bash showcase/hud.sh`. It reserves the right rail, so a maximized editor tiles
  beside it rather than under it, and it never takes keyboard focus. The panel is inset 8 px
  from the tiled client area so the window border stays visible. `bash showcase/hud.sh stop`,
  or the ✕ on the panel, releases the reservation.
- **Width**: `hud.width` in `~/.config/zmk-layer-hud/config.yaml` (598 by default); the height
  follows the board. `hud.opacity` (86) is the panel background. Restart the HUD after editing.
- **Typed keys**: the strip below the HUD, in the same rail. It shows the keys the *keyboard*
  sends — nothing typed on the laptop's built-in keyboard appears, and neither does anything a
  script injects. Characters run together into one chip, chords and special keys get their own.
- **Mode line**: the daemon's decision, large, in the empty part of the rail below the strip. The
  HUD shows layers, so RAW and OFF both read *Alpha 1*; this line is what tells them apart on
  camera. `bash showcase/modeline.sh start` puts it there (pinned floating Ghostty running
  `watch -n 0.2 -t 'zmk-vim-mode status | head -1'`); `stop` closes it. `record.py` stops it
  for beats 1, 2 and 9 — cards, diagrams and doctor carry no daemon state — and restarts it
  after those takes.

  Output: `decision : raw (code 6) — nvim client`. It is a floating window, so the rail's
  reservation does not move it; place it once and it stays.
- **Editors** are maximized on their workspaces by `prepare.sh` — never fullscreen, which hides
  layer-shell surfaces and ignores their reservations.
- **Bar widgets stripped** for every take (`prepare.sh` does it: the agent-usage pill
  `romao.agents` and the weather pill `romao.weather` leave `~/.config/omarchy/shell.json`,
  pre-strip config kept at `showcase/run/shell.json.with-widgets`, shell restarted). They
  clutter the frame and change between takes. Restore after recording:
  `cp showcase/run/shell.json.with-widgets ~/.config/omarchy/shell.json &&
  omarchy-restart-shell`. The clock stays: it is part of the frame, keep takes short
  rather than hiding it.

## Text sizes (everything ≥ 18 pt, on top of the monitor scale)

| App | Setting |
|---|---|
| Ghostty | `showcase/env/ghostty-demo.conf`: Hack Nerd Font Mono 20 |
| Neovim | inherits the terminal |
| VS Code | `demo-go.code-workspace` settings: `editor.fontSize` 18, `window.zoomLevel` 1 |
| IntelliJ | View → Appearance → Zoom IDE In to ~125 %; Settings → Editor → Font → 20 (global: revert after) |
| Obsidian | Settings → Appearance → Zoom level 125 %; vault `appearance.json` base font 20 |

## Automated takes (every beat)

`python3 showcase/record.py all` (or one beat 0–9) runs `rehearse.py`'s segment with the
rehearsal HUD, records the monitor with gpu-screen-recorder and muxes a piper TTS scratch of the
beat's narration (beat 1 is silent); takes land in `run/take<N>.mp4`. Beats 0–2 and 9 need the
keyboards repo (`KEYBOARDS_REPO`, default `~/projects/keyboards`) and an image viewer
(`ZMK_IMAGE_VIEWER`, default `imv`; `ZMK_IMAGE_VIEWER_CLASS` if its window class differs). For
the deliverable:

- `ZMK_RECORD_FPS=60 python3 showcase/record.py all` — the default is 30.
- Scale the monitor and run `bash showcase/prepare.sh` (it strips the codex/weather bar
  widgets) **before** starting; the driver does none of that, and the rehearsal HUD
  reserves its rail against whatever geometry it finds. Place the mode line after prepare.
- Stop the take HUD first (`bash showcase/hud.sh stop`): the rehearsal brings its own panel.
- The TTS track is for judging pace only; the video is dubbed with the human read (below).
- Each take ends with the rehearsal's own clean-up (rail-less frames, an empty panel): cut that
  tail. Each take also starts before the narration would: cut to the beat's *Cut* line in
  `SCRIPT.md`.

## Recording routine by hand (fallback, one beat at a time)

1. Scale the monitor (above), then `bash showcase/prepare.sh`: pristine sources,
   codex/weather widgets stripped from the bar, editors reopened clean and placed.
2. Privacy checklist (`privacy-checklist.md`), then `bash showcase/hud.sh`, then the mode line.
3. Check the HUD is live **before** the first take: type on the Diamond and watch the exact keys
   light. A HUD stuck on *waiting for the keymap…* or *waiting for the keyboard's layers…* means
   the feed never opened the device — `bash showcase/hud.sh log` says why (usually the hidraw
   udev rule). The first take ended on that empty panel.
4. Run the pre-flight list in `SCRIPT.md` (*Pre-flight, every clip*).
5. One OBS recording **per beat**, silent. Hold each state a beat longer than feels natural;
   the cut removes the slack, it cannot add it. The cards are `show <name>` in the demo shell,
   the diagrams `imv <png>`, exactly as the segments do it.
6. After each take: the mode line (or `zmk-vim-mode status`) must show the reason the script
   expects. The HUD shows the keyboard's layers, not the daemon's reason.
7. Scrub the clip before moving on: no `command not found`, no doubled text, no `(rehearsal)` chip
   in the strip, no empty HUD. Retake now, while the setup is still up.

## Sound

The first take was narrated live while typing, through the phone: pauses while thinking, slips,
a 22 kHz mono track clipping at 0 dB, and 137 s of silence between editors. Instead:

- Record every clip **silently**.
- Cut the picture first: every silence out, plain cuts between beats. No zooms, no callouts, no
  music — the HUD and the mode line are the graphics.
- Then read `SCRIPT.md` against the cut, on a USB or headset mic at 48 kHz, one beat at a time.
  Normalize to −16 LUFS (YouTube's target is −14; keep headroom), a light noise gate, no clipping.
- Trim the picture to the words, not the words to the picture.
- Reading the narration live is acceptable if each beat is its own clip and the mic is not the
  phone; the words → time table in `SCRIPT.md` is the pace either way.

## Delivery

1920×1080, 60 fps, stereo 48 kHz, no silence longer than 1.5 s, an 8 s end card and no silent
tail. Move the master over Drive or AirDrop — WhatsApp re-encodes the video and downmixes the audio.
