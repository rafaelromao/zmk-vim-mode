#!/bin/bash
# Run one segment of SCRIPT.md (or all) through Hammerspoon and print the report.
#   bash showcase/rehearse.sh 4        # Neovim segment
#   bash showcase/rehearse.sh all      # segments 3–8, about 4 minutes
#   bash showcase/rehearse.sh stop
#   VERBOSE=1 bash showcase/rehearse.sh 5   # log every step, to match beeps/popups to keys
# Keep your hands off the keyboard and mouse while it runs; every step types into the
# frontmost window. The report is showcase/run/rehearsal.log.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
HS=/opt/homebrew/bin/hs
REPORT="$HERE/run/rehearsal.log"
SEG="${1:-all}"

if ! "$HS" -c 'return 1' >/dev/null 2>&1; then
  echo "Hammerspoon IPC not loaded: add require(\"hs.ipc\") to ~/.hammerspoon/init.lua and reload." >&2
  exit 1
fi
if [ "$SEG" = "stop" ]; then
  "$HS" -c "if zmkrehearsal then zmkrehearsal.stop() end"; exit 0
fi

mkdir -p "$HERE/run"
echo "Starting segment(s): $SEG in 3 seconds. Hands off the keyboard and mouse."
sleep 3
# VERBOSE=1 logs every step (with the time), to match a beep or a popup to what was typed.
"$HS" -c "zmkrehearsal = dofile('$HERE/hud/rehearse.lua'); zmkrehearsal.run('$SEG'${VERBOSE:+, true})"

# Follow the report until DONE.
touch "$REPORT"
shown=0
while :; do
  total=$(wc -l < "$REPORT" | tr -d ' ')
  if [ "$total" -gt "$shown" ]; then
    tail -n +"$((shown + 1))" "$REPORT" | head -n "$((total - shown))"
    shown=$total
  fi
  if grep -q -E 'DONE|stopped by user' "$REPORT"; then break; fi
  sleep 0.3
done
