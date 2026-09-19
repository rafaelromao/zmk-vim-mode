#!/bin/bash
# Put the editors into a clean, known state before a rehearsal or a recording (Omarchy /
# Hyprland): committed demo content, every editor quit and reopened on the right project with
# its UI state cleared.
#
#   bash showcase/prepare.sh
#
# Editors are maximized inside the HUD's reserved work area on separate workspaces.
# Requires Hyprland's Lua dispatchers (0.55+). Launcher names:
# `code`, `idea` (Toolbox shell script) or `intellij-idea-ultimate`, `obsidian`, `ghostty`.
set -uo pipefail
SHOW="$(cd "$(dirname "$0")" && pwd)"
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
      break
    fi
    sleep 0.5
  done
  if hyprctl clients -j | jq -e '[.[] | select(.class == "code" or .class == "obsidian" or .class == "md.obsidian.Obsidian" or (.class | test("^jetbrains-idea")))] | length > 0' >/dev/null; then
    echo "Editors have not closed; resolve any save prompts before preparing again." >&2
    return 1
  fi
  # IntelliJ lingers windowless and reuses the stale instance on relaunch (seen as a
  # Welcome screen plus a "Cannot Execute Command" dialog). Wait for it to quit so the
  # relaunch below is fresh; SIGTERM only the exact launcher cmdline, never child processes.
  for _ in {1..20}; do
    pgrep -f '^/usr/share/idea/bin/idea( |$)' >/dev/null || return 0
    sleep 0.5
  done
  pkill -TERM -f '^/usr/share/idea/bin/idea( |$)' 2>/dev/null || true
  sleep 2
  pgrep -f '^/usr/share/idea/bin/idea( |$)' >/dev/null || return 0
  echo "IntelliJ did not quit; resolve it before preparing again." >&2
  return 1
}

maximize_window() {
  # Two phases: the window appears in seconds but the right project/title can lag
  # far behind (IntelliJ reuses its window on the previous project first). Place the
  # window immediately, then wait for the expected title.
  local pattern="$1" workspace="$2" project="$3"
  local address=""
  for _ in {1..240}; do
    address="$(hyprctl clients -j | jq -r --arg pattern "$pattern" \
      '[.[] | select((.class | test($pattern)) and .mapped and (.floating == false))] | sort_by(.title == "") | .[0].address // empty')"
    if [ -n "$address" ]; then
      hyprctl eval "hl.dispatch(hl.dsp.window.move({workspace='$workspace', window='address:$address'})); hl.dispatch(hl.dsp.window.fullscreen({mode='maximized', action='set', window='address:$address'}))" || return 1
      break
    fi
    sleep 0.25
  done
  [ -n "$address" ] || { echo "Could not find $project window; check $RUN launch logs." >&2; return 1; }
  # Phase 2: IDEs may open transient frames first and the project in a new window, so
  # track by class+title instead of address and re-assert placement while waiting.
  for _ in {1..240}; do
    while read -r address; do
      [ -n "$address" ] || continue
      hyprctl eval "hl.dispatch(hl.dsp.window.move({workspace='$workspace', window='address:$address'})); hl.dispatch(hl.dsp.window.fullscreen({mode='maximized', action='set', window='address:$address'}))" >/dev/null
    done < <(hyprctl clients -j | jq -r --arg pattern "$pattern" \
      '[.[] | select((.class | test($pattern)) and .mapped and (.floating == false)) | .address] | .[]')
    if hyprctl clients -j | jq -e --arg pattern "$pattern" --arg workspace "$workspace" --arg project "$project" \
      'any(.[]; ((.class | test($pattern)) and (.title | contains($project)) and .fullscreen == 1 and .workspace.id == ($workspace | tonumber)))' >/dev/null; then
      echo "  maximized: $project on workspace $workspace (HUD space retained)"
      return 0
    fi
    sleep 0.5
  done
  echo "Window placed but '$project' never appeared in its title; check $RUN launch logs." >&2
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
  # A cold `idea <path>` races the platform's async project open: the project is
  # disposed ~1s in ("Cannot Execute Command / No project was found to open the
  # file in", IdeStarter WARN in idea.log), while opening the same path into a
  # running instance over its socket works. So start bare, wait for Welcome,
  # then send the path.
  nohup "$IDEA" >"$RUN/idea.log" 2>&1 &
  for _ in {1..120}; do
    hyprctl clients -j | jq -e '[.[] | select(.class | test("^(jetbrains-idea|jetbrains-idea-ultimate|jetbrains-idea-community)$"))] | length > 0' >/dev/null && break
    sleep 0.5
  done
  hyprctl clients -j | jq -e '[.[] | select(.class | test("^(jetbrains-idea|jetbrains-idea-ultimate|jetbrains-idea-community)$"))] | length > 0' >/dev/null \
    || { echo "IntelliJ showed no window within a minute; check $RUN/idea.log." >&2; exit 1; }
  # The Welcome window exists long before the instance accepts socket opens: an
  # early `idea <path>` replays the cold-start dispose ("frame helper is not
  # found", project disposed right after indexing). Let it settle first.
  sleep 25
  nohup "$IDEA" "$SHOW/demo-java" >>"$RUN/idea.log" 2>&1 &
  maximize_window '^(jetbrains-idea|jetbrains-idea-ultimate|jetbrains-idea-community)$' 6 'demo-java' || exit 1
  # The Welcome path leaves a dead titleless surface beside the project window:
  # unfocusable, ignores close, invisible under the maximized project (the IDE
  # logs a single project frame, so this is a leaked surface, not a window).
  # Park it on workspace 9 so it can never wander on camera. Best effort only.
  parked=0
  while read -r frame; do
    [ -n "$frame" ] || continue
    if hyprctl eval "hl.dispatch(hl.dsp.window.move({workspace='9', window='address:$frame'}))" >/dev/null 2>&1; then
      parked=$((parked + 1))
    fi
  done < <(hyprctl clients -j | jq -r '[.[] | select((.class | test("^(jetbrains-idea|jetbrains-idea-ultimate|jetbrains-idea-community)$")) and .title == "") | .address] | .[]')
  [ "$parked" -gt 0 ] && echo "  intellij: parked $parked untitled frame(s) on workspace 9"
  true
} || echo "  intellij launcher not found; open showcase/demo-java by hand"
hyprctl dispatch 'hl.dsp.focus({workspace="7"})' || exit 1
# --in-process-gpu: this NVIDIA box kills Electron's separate GPU process
# ("GPU process isn't usable. Goodbye."). Command line only; user flags untouched.
nohup obsidian --in-process-gpu "obsidian://open?vault=$VAULT_NAME&file=Tasks" >"$RUN/obsidian.log" 2>&1 &
maximize_window '^(obsidian|md\.obsidian\.Obsidian)$' 7 ' - Demo - ' || exit 1

step "daemon state"
if command -v zmk-vim-mode >/dev/null 2>&1; then
  if [ "$(zmk-vim-mode status --json 2>/dev/null | jq -r '.override == null')" != "true" ]; then
    echo "  WARNING: a mode override is active; clear it before recording or the daemon stays forced" >&2
  else
    echo "  daemon in auto mode"
  fi
fi

step "workspace layout (one editor per workspace, maximized)"
layout_ok=1
check_ws() {  # $1 = class pattern, $2 = workspace, $3 = title fragment
  n=$(hyprctl clients -j | jq --arg p "$1" --argjson w "$2" --arg t "$3" \
    '[.[] | select((.class | test($p)) and .workspace.id == $w and (.title | contains($t)) and .fullscreen == 1)] | length')
  if [ "$n" -ge 1 ]; then
    echo "  ok: $3 maximized on workspace $2"
  else
    echo "  LAYOUT PROBLEM: $3 not maximized on workspace $2" >&2
    layout_ok=0
  fi
}
check_ws '^code$' 5 'demo-go'
check_ws '^(jetbrains-idea|jetbrains-idea-ultimate|jetbrains-idea-community)$' 6 'demo-java'
check_ws '^(obsidian|md\.obsidian\.Obsidian)$' 7 ' - Demo - '
[ "$layout_ok" = 1 ] || exit 1

cat <<EOF

Ready once IntelliJ has indexed. Then:
  bash showcase/hud.sh                # the layer HUD (zmk-layer-hud, reads the keyboard)
  python3 showcase/rehearse.py all    # or a single segment 3-8
EOF
