#!/bin/bash
# Linux (Omarchy / Hyprland) host for the showcase HUD: the same two pages as on macOS,
# shown as floating, pinned, unfocusable browser app windows and driven by keyfeed.py.
#
#   bash showcase/linux/hud.sh          start (rebuilds keymap data, starts keyfeed.py)
#   bash showcase/linux/hud.sh stop
#
# Needs: python-evdev python-websockets, a Chromium-based browser (chromium, brave, google-chrome)
# for --app windows, jq, hyprctl. UNTESTED as of the handoff (written on macOS).
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
HUD="$HERE/../hud"
RUN="$HERE/../run"
PORT="${ZMKHUD_PORT:-8766}"
MARGIN=24
HUD_W=598; HUD_H=392
STRIP_W=900; STRIP_H=72
mkdir -p "$RUN"

stop() {
  pkill -f "keyfeed.py" 2>/dev/null || true
  pkill -f -- "--class=zmkhud" 2>/dev/null || true
  hyprctl keyword windowrulev2 "unset,class:^(zmkhud)$" >/dev/null 2>&1 || true
  echo "HUD stopped"
}
[ "${1:-start}" = "stop" ] && { stop; exit 0; }

BROWSER="$(command -v chromium || command -v brave || command -v google-chrome-stable || command -v google-chrome || true)"
[ -n "$BROWSER" ] || { echo "no Chromium-based browser found (chromium/brave/google-chrome)" >&2; exit 1; }

python3 "$HUD/../keymap/build.py"
stop >/dev/null 2>&1 || true

# The recording monitor: the one that is not the laptop panel (eDP-*), else the focused one.
MON_JSON="$(hyprctl monitors -j)"
MON="$(echo "$MON_JSON" | jq -r '[.[] | select(.name | test("^eDP") | not)][0] // .[0]')"
MX=$(echo "$MON" | jq -r .x); MY=$(echo "$MON" | jq -r .y)
MW=$(echo "$MON" | jq -r '(.width / .scale) | floor'); MH=$(echo "$MON" | jq -r '(.height / .scale) | floor')
echo "recording monitor: $(echo "$MON" | jq -r .name) ${MW}x${MH} at ${MX},${MY}"

# Window rules for both pages: floating, pinned to every workspace, never focused, no border.
for r in "float" "pin" "noinitialfocus" "nofocus" "noborder" "noshadow" "noblur" "opaque"; do
  hyprctl keyword windowrulev2 "$r,class:^(zmkhud)$" >/dev/null
done
hyprctl keyword windowrulev2 "move $((MX + MW - HUD_W - MARGIN)) $((MY + MARGIN)),class:^(zmkhud)$,title:^(ZMK layer HUD)$" >/dev/null
hyprctl keyword windowrulev2 "size $HUD_W $HUD_H,class:^(zmkhud)$,title:^(ZMK layer HUD)$" >/dev/null
hyprctl keyword windowrulev2 "move $((MX + MARGIN)) $((MY + MH - STRIP_H - MARGIN)),class:^(zmkhud)$,title:^(typed keys)$" >/dev/null
hyprctl keyword windowrulev2 "size $STRIP_W $STRIP_H,class:^(zmkhud)$,title:^(typed keys)$" >/dev/null
# Keep tiled editors above the strip: reserve the bottom band on the recording monitor.
hyprctl keyword monitor "$(echo "$MON" | jq -r .name),addreserved,0,$((STRIP_H + 2 * MARGIN)),0,0" >/dev/null || true

nohup python3 "$HERE/keyfeed.py" >"$RUN/keyfeed.log" 2>&1 &
sleep 1

PROFILE="$RUN/chromium-hud"
for page in "index.html" "keys.html"; do
  nohup "$BROWSER" --app="file://$HUD/$page?ws=ws://127.0.0.1:$PORT" --class=zmkhud \
    --user-data-dir="$PROFILE" --no-first-run --disable-features=Translate \
    --enable-features=UseOzonePlatform --ozone-platform=wayland >/dev/null 2>&1 &
  sleep 0.5
done

echo "HUD started (log: $RUN/keyfeed.log). Stop with: $0 stop"
