#!/bin/bash
# Put the editors into a clean, known state before a rehearsal or a recording (macOS):
# committed demo content, every editor quit and reopened on the right project with no
# leftover tabs, panels or tool windows, windows placed on the recording display.
#
#   bash showcase/prepare.sh
#
# Runs in your terminal (needs the `hs` CLI and `osascript`). ~30 s, mostly IntelliJ starting.
# If an editor asks about unsaved changes, answer "Don't Save": the content is reset anyway.
# The Linux counterpart is showcase/linux/prepare.sh.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
HS=/opt/homebrew/bin/hs
VAULT_NAME="Demo"                 # showcase/Demo, registered in Obsidian under that folder name

step() { printf '\n▸ %s\n' "$*"; }

step "demo content back to the committed state"
bash "$HERE/setup.sh" >/dev/null 2>&1 || bash "$HERE/setup.sh"

step "quitting the editors"
for app in "Visual Studio Code" "IntelliJ IDEA" "Obsidian"; do
  osascript -e "tell application \"$app\" to quit" >/dev/null 2>&1 || true
done
# the demo Ghostty instance (separate process, launched with our config file)
pkill -f 'ghostty-demo.conf' 2>/dev/null || true
sleep 3

step "forgetting per-project UI state (open tabs, panels, tool windows, view mode)"
# VS Code keeps restored editors and panel layout per workspace under workspaceStorage.
CODE_STORAGE="$HOME/Library/Application Support/Code/User/workspaceStorage"
if [ -d "$CODE_STORAGE" ]; then
  grep -l "demo-go.code-workspace" "$CODE_STORAGE"/*/workspace.json 2>/dev/null | while read -r f; do
    rm -rf "$(dirname "$f")" && echo "  vscode: cleared $(basename "$(dirname "$f")")"
  done
fi
# IntelliJ keeps open editors and tool windows in .idea/workspace.xml.
rm -f "$HERE/demo-java/.idea/workspace.xml" 2>/dev/null && echo "  intellij: cleared .idea/workspace.xml"
# Obsidian keeps the open tabs and their view mode (editing/reading) in workspace.json.
rm -f "$HERE/Demo/.obsidian/workspace.json" 2>/dev/null && echo "  obsidian: cleared Demo/.obsidian/workspace.json"

step "reopening on the demo projects"
open -a "Visual Studio Code" "$HERE/demo-go.code-workspace"
open -a "IntelliJ IDEA" "$HERE/demo-java"
open "obsidian://open?vault=$VAULT_NAME&file=Tasks"
sleep 8
# VS Code: the file the script uses, on line 1
code -r --goto "$HERE/demo-go/internal/modes/modes.go:1:1" >/dev/null 2>&1 || true

step "placing the windows on the recording display"
if "$HS" -c 'return 1' >/dev/null 2>&1; then
  n=0
  for _ in 1 2 3 4; do
    n="$("$HS" -c "return dofile('$HERE/hud/place.lua').all()" 2>/dev/null || echo 0)"
    [ "${n:-0}" -ge 3 ] && break
    sleep 3
  done
  echo "  placed $n window(s)"
else
  echo "  hs CLI not available; place the windows by hand"
fi

cat <<EOF

Ready. IntelliJ may still be indexing for a moment. Then:
  bash showcase/hud/start.sh        # HUD + typed-keys strip (also quits KeyCastr)
  bash showcase/rehearse.sh all     # or a single segment 3–8
EOF
