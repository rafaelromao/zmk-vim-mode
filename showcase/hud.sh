#!/bin/bash
# Start / stop the layer HUD for a take. The HUD is its own project (rafaelromao/zmk-layer-hud):
# the keyboard reports its own layers and key positions on its signal channel (CDC-ACM over USB,
# GATT over BLE) and the host reads what is typed from the keyboard's HID reports, so the banner
# and the lit keys are what the Diamond really did. This script only finds that checkout, checks
# the obvious things and hands over.
#
#   bash showcase/hud.sh            start, reserving the right rail (takes)
#   bash showcase/hud.sh --overlay  start as an overlay (no reservation)
#   bash showcase/hud.sh stop       (or the ✕ on the panel)
#   bash showcase/hud.sh log        tail the panel and feed logs
#
# Checkout: $ZMK_LAYER_HUD, default ~/projects/zmk-layer-hud. Config: $ZMKHUD_CONFIG, default
# ~/.config/zmk-layer-hud/config.yaml. One-time setup is in that repo's README: the ZMK module
# on the keyboard (`positions;`, built with the layer-hud-usb-uart snippet), the venv and the
# udev rule for the tty + hidraw -- `zmk-layer-hud setup` does all three. Rehearsals do not use this:
# showcase/rehearse.py runs its own HUD, because synthesized keys never reach the keyboard.
set -euo pipefail

HUD="${ZMK_LAYER_HUD:-$HOME/projects/zmk-layer-hud}"
CONFIG="${ZMKHUD_CONFIG:-$HOME/.config/zmk-layer-hud/config.yaml}"
CMD="start"
RESERVE="--reserve"
for arg in "$@"; do
  case "$arg" in
    start|stop|log) CMD="$arg" ;;
    --reserve) RESERVE="--reserve" ;;
    --overlay) RESERVE="" ;;
    *) echo "usage: $0 [start|stop|log] [--reserve|--overlay]" >&2; exit 2 ;;
  esac
done

if [ "$(uname -s)" != Linux ]; then
  echo "This kit records on Omarchy; the HUD host here is Linux-only." >&2
  echo "On another platform, run the host of $HUD directly (see its README)." >&2
  exit 1
fi

if [ ! -d "$HUD" ]; then
  cat >&2 <<MSG
No zmk-layer-hud checkout at $HUD.

    git clone https://github.com/rafaelromao/zmk-layer-hud "$HUD"

or point ZMK_LAYER_HUD at yours.
MSG
  exit 1
fi

HOST="$HUD/host/linux/hud.sh"
[ -f "$HOST" ] || { echo "$HOST is missing; update the checkout at $HUD" >&2; exit 1; }

# The logs left the checkout when that project grew a single command: they are under
# $ZMKHUD_STATE now (default ~/.local/state/zmk-layer-hud), so that `zmk-layer-hud update` can
# replace the tree without taking them with it. `zmk-layer-hud log` is this same tail.
STATE="${ZMKHUD_STATE:-${XDG_STATE_HOME:-$HOME/.local/state}/zmk-layer-hud}"
if [ "$CMD" = log ]; then
  logs=()
  for f in "$STATE/panel.log" "$STATE/hudfeed.log"; do [ -f "$f" ] && logs+=("$f"); done
  [ "${#logs[@]}" -gt 0 ] || { echo "no logs in $STATE yet; start the HUD first" >&2; exit 1; }
  exec tail -n 40 -f "${logs[@]}"
fi

if [ "$CMD" = start ]; then
  if [ ! -f "$CONFIG" ]; then
    cat >&2 <<MSG
No HUD config at $CONFIG.

    mkdir -p "$(dirname "$CONFIG")" && cp "$HUD/config/diamond.yaml" "$CONFIG"

then check the paths it points at (the keymap-drawer YAML and src/definitions/config.dtsi in
~/projects/keyboards). Set ZMKHUD_CONFIG to use another file.
MSG
    exit 1
  fi
  if [ -z "${ZMKHUD_PYTHON:-}" ] && [ ! -x "$HUD/.venv/bin/python3" ]; then
    cat >&2 <<MSG
No virtualenv in $HUD. Once:

    $HUD/bin/zmk-layer-hud setup

which installs the system packages, builds the venv and offers the udev rule, printing every
privileged step and asking before it runs.
MSG
    exit 1
  fi
fi

# That host validates the config and the keymap-drawer YAML itself, with a readable error.
# Takes reserve the right rail (editors tile beside the HUD); the upstream default is an
# overlay, which would leave editors under the board on camera.
# shellcheck disable=SC2086
exec bash "$HOST" "$CMD" $RESERVE
