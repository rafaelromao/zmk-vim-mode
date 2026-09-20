#!/bin/bash
# The mode line for the showcase takes: one large terminal line with the daemon's
# decision (`zmk-vim-mode status | head -1`). The HUD shows the keyboard's layers;
# this shows the daemon's reason, which is what tells RAW from OFF on camera.
#
#   bash showcase/modeline.sh start   # pinned floating Ghostty in the rail, below the strip
#   bash showcase/modeline.sh stop    # close it
#
# Fixed geometry (physical pixels at the 1.25 recording scale): x1431 y558, 617 wide,
# in the HUD's right rail under the typed-keys strip (strip ends at y550). Pinned, so it
# rides every workspace; floating, so the rail reservation never moves it. record.py stops
# it for beats 1, 2 and 9 — cards, diagrams and doctor carry no daemon state, and an
# "off — terminal without nvim client" line there is noise — and restarts it after.
set -euo pipefail
SHOW="$(cd "$(dirname "$0")" && pwd)"
TITLE="zmk-modeline"

addr() {
  hyprctl clients -j | jq -r --arg t "$TITLE" \
    '[.[] | select(.title == $t)] | .[0].address // empty'
}

if [ "${1:-start}" = stop ]; then
  a="$(addr)"
  if [ -n "$a" ]; then
    hyprctl eval "hl.dispatch(hl.dsp.window.close({window='address:$a'}))" >/dev/null
  fi
  exit 0
fi

[ -n "$(addr)" ] && { echo "mode line already up"; exit 0; }
nohup ghostty --gtk-single-instance=false \
  --config-file="$SHOW/env/ghostty-demo.conf" \
  --font-size=16 --window-decoration=false --title="$TITLE" \
  -e /bin/bash -c "watch -n 0.2 -t 'zmk-vim-mode status | head -1'" \
  >/dev/null 2>&1 &
sleep 4
a="$(hyprctl clients -j | jq -r --arg t "$TITLE" \
  '[.[] | select(.title == $t)] | .[0].address // empty')"
[ -n "$a" ] || { echo "mode line window never appeared" >&2; exit 1; }
hyprctl eval "hl.dispatch(hl.dsp.window.float({window='address:$a', action='toggle'}))" >/dev/null
# Fresh windows can land maximized or fullscreen depending on the workspace state;
# clear both (ignore the one that errors), or resize/move/pin refuse.
hyprctl eval "hl.dispatch(hl.dsp.window.fullscreen({mode='maximized', action='unset', window='address:$a'}))" >/dev/null 2>&1 || true
hyprctl eval "hl.dispatch(hl.dsp.window.fullscreen({mode='fullscreen', action='unset', window='address:$a'}))" >/dev/null 2>&1 || true
sleep 1
hyprctl eval "hl.dispatch(hl.dsp.window.resize({window='address:$a', x=617, y=100}))" >/dev/null
sleep 1
# The move races an in-flight resize: repeat until the window reports the target.
for _ in 1 2 3 4; do
  hyprctl eval "hl.dispatch(hl.dsp.window.move({window='address:$a', x=1431, y=558}))" >/dev/null
  sleep 1
  [ "$(hyprctl clients -j | jq -r --arg a "$a" \
    '.[] | select(.address == $a) | "\(.at[0]),\(.at[1])"')" = "1431,558" ] && break
done
hyprctl eval "hl.dispatch(hl.dsp.window.pin({window='address:$a'}))" >/dev/null
echo "mode line at $(hyprctl clients -j | jq -r --arg a "$a" \
  '.[] | select(.address == $a) | "\(.at) \(.size) pin=\(.pinned)"')"
