#!/bin/bash
# Linux (Omarchy / Hyprland) counterpart of showcase/prepare.sh: committed demo content, every
# editor quit and reopened on the right project with its UI state cleared.
#
#   bash showcase/linux/prepare.sh
#
# Window placement is left to Hyprland's tiling plus the reserved bottom band that hud.sh sets.
# UNTESTED as of the handoff (written on macOS). Adjust the launcher names to the box:
# `code`, `idea` (Toolbox shell script) or `intellij-idea-ultimate`, `obsidian`, `ghostty`.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
SHOW="$(cd "$HERE/.." && pwd)"
VAULT_NAME="Demo"

step() { printf '\n▸ %s\n' "$*"; }

step "demo content back to the committed state"
bash "$SHOW/setup.sh" >/dev/null 2>&1 || bash "$SHOW/setup.sh"

step "quitting the editors"
pkill -x code 2>/dev/null || pkill -f 'code --' 2>/dev/null || true
pkill -f 'idea' 2>/dev/null || true
pkill -x obsidian 2>/dev/null || pkill -f 'obsidian' 2>/dev/null || true
pkill -f 'ghostty-demo.conf' 2>/dev/null || true
sleep 3

step "forgetting per-project UI state"
for storage in "$HOME/.config/Code/User/workspaceStorage" "$HOME/.config/Code - OSS/User/workspaceStorage" "$HOME/.config/VSCodium/User/workspaceStorage"; do
  [ -d "$storage" ] || continue
  grep -l "demo-go.code-workspace" "$storage"/*/workspace.json 2>/dev/null | while read -r f; do
    rm -rf "$(dirname "$f")" && echo "  vscode: cleared $(basename "$(dirname "$f")")"
  done
done
rm -f "$SHOW/demo-java/.idea/workspace.xml" 2>/dev/null && echo "  intellij: cleared .idea/workspace.xml"
rm -f "$SHOW/Demo/.obsidian/workspace.json" 2>/dev/null && echo "  obsidian: cleared Demo/.obsidian/workspace.json"

step "reopening on the demo projects"
(code "$SHOW/demo-go.code-workspace" --goto "$SHOW/demo-go/internal/modes/modes.go:1:1" >/dev/null 2>&1 &)
IDEA="$(command -v idea || command -v intellij-idea-ultimate || command -v intellij-idea-community || true)"
[ -n "$IDEA" ] && ("$IDEA" "$SHOW/demo-java" >/dev/null 2>&1 &) || echo "  intellij launcher not found; open showcase/demo-java by hand"
(xdg-open "obsidian://open?vault=$VAULT_NAME&file=Tasks" >/dev/null 2>&1 &)

cat <<EOF

Ready once IntelliJ has indexed. Then:
  bash showcase/linux/hud.sh          # HUD + typed-keys strip
  python3 showcase/linux/rehearse.py all
EOF
