#!/bin/bash
# Native transparent Wayland HUD and reserved typed-keys panel.
# Arch dependencies: python-gobject webkit2gtk-4.1 gtk-layer-shell python-evdev python-websockets
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
RUN="$HERE/../run"
mkdir -p "$RUN"

stop() {
  # Also retire overlays started by the previous Chromium host.
  pkill -f -- "--class=zmkhud" 2>/dev/null || true
  pkill -f "$HERE/panel.py" 2>/dev/null || true
  pkill -f "$HERE/keyfeed.py" 2>/dev/null || true
  hyprctl eval 'if zmkhud_rules then for _, r in ipairs(zmkhud_rules) do r:set_enabled(false) end end' >/dev/null 2>&1 || true
}

if [ "${1:-start}" = stop ]; then
  stop
  echo "HUD stopped; panel reservation released"
  exit 0
fi
python3 -c "import evdev, websockets, gi; gi.require_version('Gtk', '3.0'); gi.require_version('WebKit2', '4.1'); gi.require_version('GtkLayerShell', '0.1')"
python3 "$HERE/../keymap/build.py"
stop
sleep 0.5
nohup python3 -u "$HERE/panel.py" >"$RUN/panel.log" 2>&1 &
PID=$!
sleep 2
kill -0 "$PID" 2>/dev/null || { echo "HUD failed; see $RUN/panel.log" >&2; exit 1; }
echo "HUD started; HUD reserves the right rail, typed keys the bottom panel. Stop with: $0 stop"
