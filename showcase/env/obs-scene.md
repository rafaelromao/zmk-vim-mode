# OBS scene for the showcase

## Displays (from `displayplacer list`, 2026-09-15)

| Display | id | Current mode | Role |
|---|---|---|---|
| MacBook built-in, main | `37D8832A-2D66-02CA-B9F7-8F30A301B230` | 1512×982 HiDPI (3024×1964 panel) | OBS, notes, this script |
| 27" external, right of the laptop | `1B2458CE-ADBD-4AD9-9827-FBB82B823C77` | 2560×1440, scaling off | **recording display** |

Both displays stay on: the recording display holds only the editor, the typed-keys strip and the HUD;
OBS and the script live on the laptop. Start the HUD with the mouse on the 27" (it opens on
the screen under the mouse) or `hs -c "zmkhud.moveTo(hs.screen.allScreens()[2])"`.

## Canvas and output

**Recommended: HiDPI 1920×1080 on the 27".** Mode 34 renders a 3840×2160 framebuffer and
scales it to the panel, so macOS UI is 33 % larger without touching any app setting, and OBS
captures the 4K framebuffer and downsamples it 2:1 — the crispest 1080p you can get. The
panel itself looks slightly soft to *you* (1.5× scaling); the recording does not.

```bash
# before recording
displayplacer "id:1B2458CE-ADBD-4AD9-9827-FBB82B823C77 res:1920x1080 hz:60 color_depth:8 scaling:on origin:(1512,0) degree:0"
# after recording (the arrangement `displayplacer list` printed)
displayplacer "id:37D8832A-2D66-02CA-B9F7-8F30A301B230 res:1512x982 hz:120 color_depth:8 enabled:true scaling:on origin:(0,0) degree:0" "id:1B2458CE-ADBD-4AD9-9827-FBB82B823C77 res:2560x1440 hz:60 color_depth:8 enabled:true scaling:off origin:(1512,0) degree:0"
```

| Setting | HiDPI route (recommended) | Native route |
|---|---|---|
| 27" mode | 1920×1080 `scaling:on` (mode 34) | 2560×1440 `scaling:off` (current, mode 27) |
| Settings → Video → Base (canvas) | 1920×1080 (the 3840×2160 source is fitted to it, *Lanczos*) | 2560×1440 |
| Settings → Video → Output (scaled) | 1920×1080 | **1920×1080**, Lanczos |
| Text sizes | app defaults already read at 1.33×; keep the sizes below anyway | sizes below are mandatory |
| FPS | 60 | 60 |
| Settings → Output → Recording | MP4 (or MKV then remux), Apple VT H.264 CQ ~18, or ProRes if you edit afterwards | same |
| Audio | mic on its own track, desktop audio muted | same |

At 60 fps the HUD's ~300 ms key flashes read cleanly; at 30 they stutter.

Laptop-only fallback (no external): `displayplacer "id:37D8832A-2D66-02CA-B9F7-8F30A301B230 res:1920x1200 hz:60 color_depth:8 scaling:on"`,
canvas 1920×1200 with a 60 px top/bottom crop filter on the display source, then restore
1512×982 with the command above.

## Sources (top to bottom)

1. **Display Capture** — pick the *27 inch* display (display 2), cursor shown. Grant OBS
   Screen Recording on first use.
2. Nothing else. The HUD and its typed-keys strip are real windows on that display, so
   they are captured as-is and you see exactly what the viewer sees while presenting.

## Window layout on the recording display (1920×1080 points on the HiDPI route)

```
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│   editor / terminal, maximised (menu bar auto-hidden)        │
│                                                              │
│                                                              │
│                                                              │
│                                          ┌────────────────┐  │
│  (HUD is top-right, 24 pt margins)       │ layer HUD      │  │
│                                          │ 598×392 pt     │  │
│  ┌──────────────┐                        └────────────────┘  │
│  │ typed keys   │   bottom band, 120 pt                      │
│  └──────────────┘                                            │
└──────────────────────────────────────────────────────────────┘
```

- **HUD**: `bash showcase/hud/start.sh` puts it 24 pt from the top-right
  corner of the recording display. Move it with
  `hs -c "zmkhud.moveTo(hs.screen.allScreens()[2])"` if it picked the wrong one.
  It opens at 598×392 pt; shrink it with `hs -c "zmkhud.resize(480, 315)"` if it covers too much.
- **Typed keys**: drawn by the HUD itself (`hud/keys.html`) bottom-left of the same
  display, fed by Hammerspoon's event tap: characters run together into one chip,
  chords and special keys get their own, everything fades after 1.8 s. KeyCastr is
  no longer needed; quit it so the keys are not shown twice.
- Editors fill the display minus a 120 pt band at the bottom, where the typed-keys strip
  lives (`prepare.sh` and the rehearsal size them that way). The HUD sits top-right over the
  editor by design; set a plain wallpaper so the band below the editors is a clean colour.

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
2. Privacy checklist (`privacy-checklist.md`), then `bash showcase/hud/start.sh`.
3. One OBS recording **per segment** of `SCRIPT.md`; say the segment number
   before starting to type so the cut is easy.
4. After each take: `zmk-vim-mode status` must show the reason the script expects.
5. Narration: record per segment while watching the take, or read the script
   live — the script's timings assume ~140 words per minute.
