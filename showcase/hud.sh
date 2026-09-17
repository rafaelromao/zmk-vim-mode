#!/bin/bash
# Start / stop the layer HUD for a take. The HUD is its own project (rafaelromao/zmk-layer-hud):
# it reads the keyboard's own HID reports, so the layers and the lit keys are what the Diamond
# really did. This script only finds that checkout, checks the obvious things and hands over.
#
#   bash showcase/hud.sh            start
#   bash showcase/hud.sh stop       (or the ✕ on the panel)
#   bash showcase/hud.sh log        tail the panel and feed logs
#
# Checkout: $ZMK_LAYER_HUD, default ~/projects/zmk-layer-hud. Config: $ZMKHUD_CONFIG, default
# ~/.config/zmk-layer-hud/config.yaml. One-time setup is in that repo's README: the ZMK module
# on the keyboard (`positions;` + CONFIG_ZMK_HID_KEYBOARD_REPORT_SIZE=12), the venv, the udev
# rule for hidraw. Rehearsals do not use this: showcase/rehearse.py runs its own HUD, because
# synthesized keys never reach the keyboard.
set -euo pipefail

HUD="${ZMK_LAYER_HUD:-$HOME/projects/zmk-layer-hud}"
CONFIG="${ZMKHUD_CONFIG:-$HOME/.config/zmk-layer-hud/config.yaml}"
CMD="${1:-start}"

case "$CMD" in start|stop|log) ;; *) echo "usage: $0 [start|stop|log]" >&2; exit 2 ;; esac

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

# `log` has no verb in the Linux host: tail the same two files from here.
if [ "$CMD" = log ]; then
  logs=()
  for f in "$HUD/run/panel.log" "$HUD/run/hudfeed.log"; do [ -f "$f" ] && logs+=("$f"); done
  [ "${#logs[@]}" -gt 0 ] || { echo "no logs in $HUD/run yet; start the HUD first" >&2; exit 1; }
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
No virtualenv in $HUD. In that repo, once:

    sudo pacman -S python-gobject webkit2gtk-4.1 gtk-layer-shell
    make venv && .venv/bin/pip install websockets
    sudo cp contrib/udev/60-zmk-layer-hud.rules /etc/udev/rules.d/ &&
      sudo udevadm control --reload-rules && sudo udevadm trigger
MSG
    exit 1
  fi
fi

# That host validates the config and the keymap-drawer YAML itself, with a readable error.
exec bash "$HOST" "$CMD"
