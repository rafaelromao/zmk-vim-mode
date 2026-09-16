#!/bin/bash
# Linux (Omarchy / Hyprland) counterpart of showcase/prepare.sh: committed demo content, every
# editor quit and reopened on the right project with its UI state cleared.
#
#   bash showcase/linux/prepare.sh
#
# Editors are maximized inside the HUD's reserved work area on separate workspaces.
# Requires Hyprland's Lua dispatchers (0.55+). Launcher names:
# `code`, `idea` (Toolbox shell script) or `intellij-idea-ultimate`, `obsidian`, `ghostty`.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
SHOW="$(cd "$HERE/.." && pwd)"
VAULT_NAME="Demo"
RUN="$SHOW/run"
mkdir -p "$RUN"

step() { printf '\n▸ %s\n' "$*"; }

close_editor_windows() {
  hyprctl clients -j | jq -r '.[] | select(.class == "code" or .class == "obsidian" or .class == "md.obsidian.Obsidian" or (.class | test("^jetbrains-idea"))) | .address' |
    while read -r address; do
      [ -n "$address" ] || continue
      hyprctl eval "return hl.dispatch(hl.dsp.window.close({window='address:$address'}))" >/dev/null
    done
  for _ in {1..60}; do
    if hyprctl clients -j | jq -e '[.[] | select(.class == "code" or .class == "obsidian" or .class == "md.obsidian.Obsidian" or (.class | test("^jetbrains-idea")))] | length == 0' >/dev/null; then
      sleep 2
      return 0
    fi
    sleep 0.5
  done
  echo "Editors have not closed; resolve any save prompts before preparing again." >&2
  return 1
}

maximize_window() {
  local pattern="$1" workspace="$2" project="$3"
  local address
  for _ in {1..240}; do
    address="$(hyprctl clients -j | jq -r --arg pattern "$pattern" --arg project "$project" \
      '[.[] | select((.class | test($pattern)) and (.title | contains($project)) and .mapped and (.floating == false)) | .address][0] // empty')"
    if [ -n "$address" ]; then
      hyprctl eval "hl.dispatch(hl.dsp.window.move({workspace='$workspace', window='address:$address'})); hl.dispatch(hl.dsp.window.fullscreen({mode='maximized', action='set', window='address:$address'}))" || return 1
      sleep 1
      if hyprctl clients -j | jq -e --arg address "$address" --arg workspace "$workspace" \
        'any(.[]; .address == $address and .fullscreen == 1 and .workspace.id == ($workspace | tonumber))' >/dev/null; then
        echo "  maximized: $project on workspace $workspace (HUD space retained)"
        return 0
      fi
    fi
    sleep 0.25
  done
  echo "Could not verify maximized $project; check $RUN launch logs." >&2
  return 1
}

step "quitting the editors"
close_editor_windows || exit 1

step "demo content back to the committed state"
bash "$SHOW/setup.sh" || exit 1

step "forgetting per-project UI state"
# Preserve VS Code workspace storage: it can contain recovery state for unsaved tabs.
rm -f "$SHOW/demo-java/.idea/workspace.xml" 2>/dev/null && echo "  intellij: cleared .idea/workspace.xml"
rm -f "$SHOW/Demo/.obsidian/workspace.json" 2>/dev/null && echo "  obsidian: cleared Demo/.obsidian/workspace.json"

step "reopening each editor maximized beside the HUD"
hyprctl dispatch 'hl.dsp.focus({workspace="5"})' || exit 1
nohup code --new-window "$SHOW/demo-go.code-workspace" --goto "$SHOW/demo-go/internal/modes/modes.go:1:1" >"$RUN/code.log" 2>&1 &
maximize_window '^code$' 5 'demo-go' || exit 1
IDEA="$(command -v idea || command -v intellij-idea-ultimate || command -v intellij-idea-community || true)"
[ -n "$IDEA" ] && {
  hyprctl dispatch 'hl.dsp.focus({workspace="6"})' || exit 1
  nohup "$IDEA" "$SHOW/demo-java" >"$RUN/idea.log" 2>&1 &
  maximize_window '^(jetbrains-idea|jetbrains-idea-ultimate|jetbrains-idea-community)$' 6 'demo-java' || exit 1
} || echo "  intellij launcher not found; open showcase/demo-java by hand"
hyprctl dispatch 'hl.dsp.focus({workspace="7"})' || exit 1
nohup obsidian "obsidian://open?vault=$VAULT_NAME&file=Tasks" >"$RUN/obsidian.log" 2>&1 &
maximize_window '^(obsidian|md\.obsidian\.Obsidian)$' 7 ' - Demo - ' || exit 1

cat <<EOF

Ready once IntelliJ has indexed. Then:
  bash showcase/linux/hud.sh          # HUD + typed-keys strip
  python3 showcase/linux/rehearse.py all
EOF
