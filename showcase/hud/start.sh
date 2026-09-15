#!/bin/bash
# Start / stop the layer HUD in the running Hammerspoon.
#   showcase/hud/start.sh        start (rebuilds keymap.json first)
#   showcase/hud/start.sh stop
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
HS=/opt/homebrew/bin/hs

if ! command -v "$HS" >/dev/null; then
  echo "hs CLI not found; Hammerspoon installs it at $HS" >&2; exit 1
fi
if ! "$HS" -c 'return 1' >/dev/null 2>&1; then
  cat >&2 <<MSG
Hammerspoon's IPC port is not loaded. Add this line to ~/.hammerspoon/init.lua and reload Hammerspoon:

    require("hs.ipc")

MSG
  exit 1
fi

case "${1:-start}" in
  start)
    python3 "$HERE/../keymap/build.py"
    # Errors from hud.lua are printed here, not swallowed.
    "$HS" -c "if zmkhud then zmkhud.stop() end; zmkhud = dofile('$HERE/hud.lua'); return zmkhud.selftest()"
    echo "HUD started on the recording display. Stop with: $0 stop"
    ;;
  stop) "$HS" -c "if zmkhud then zmkhud.stop() end" ;;
  *) echo "usage: $0 [start|stop]" >&2; exit 2 ;;
esac
