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

Record at the monitor's native mode and let OBS scale to 1080p. On Hyprland, a fractional
`scale` enlarges every app at once (the counterpart of the old HiDPI trick); `hyprctl keyword
monitor NAME,PREFERRED,auto,1.25` tries one live, and `hyprctl reload` puts your config back.

| Setting | Value |
|---|---|
| Settings → Video → Base (canvas) | the recording monitor's mode (e.g. 2560×1440) |
| Settings → Video → Output (scaled) | 1920×1080, Lanczos |
| FPS | 60 |
| Settings → Output → Recording | MP4 (or MKV then remux), H.264 (VAAPI/NVENC) CQ ~18 |
| Audio | mic on its own track, desktop audio muted |
| Text sizes | the table below is mandatory at 1440p |

At 60 fps the HUD's key flashes read cleanly; at 30 they stutter. The flash is `hud.press_ms`
(320 ms) in `~/.config/zmk-layer-hud/config.yaml`, with `hud.combo_pill_ms` (1000 ms) for the
combo pill — raise them if the cut is faster than the eye.

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
│                                        ┆                     │
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
- **Editors** are maximized on their workspaces by `prepare.sh` — never fullscreen, which hides
  layer-shell surfaces and ignores their reservations.

## Text sizes (everything ≥ 18 pt at 1440p)

| App | Setting |
|---|---|
| Ghostty | `showcase/env/ghostty-demo.conf`: Hack Nerd Font Mono 20 |
| Neovim | inherits the terminal |
| VS Code | `demo-go.code-workspace` settings: `editor.fontSize` 18, `window.zoomLevel` 1 |
| IntelliJ | View → Appearance → Zoom IDE In to ~125 %; Settings → Editor → Font → 20 (global: revert after) |
| Obsidian | Settings → Appearance → Zoom level 125 %; vault `appearance.json` base font 20 |

## Recording routine

1. `bash showcase/prepare.sh`: pristine sources, editors reopened clean and placed.
2. Privacy checklist (`privacy-checklist.md`), then `bash showcase/hud.sh`.
3. Check the HUD is live **before** the first take: type on the Diamond and watch the exact keys
   light. A HUD stuck on "waiting for the keyboard's layers…" means the feed never opened the
   device — `bash showcase/hud.sh log` says why (usually the hidraw udev rule).
4. One OBS recording **per segment** of `SCRIPT.md`; say the segment number before starting to
   type so the cut is easy.
5. After each take: `zmk-vim-mode status` must show the reason the script expects. The HUD shows
   the keyboard's layers, not the daemon's reason, so `status` is the only judge.
6. Narration: record per segment while watching the take, or read the script live — the script's
   timings assume ~140 words per minute.
