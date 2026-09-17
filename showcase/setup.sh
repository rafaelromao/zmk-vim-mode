#!/bin/bash
# Reset the showcase content to what is committed, and create the runtime directory.
#
#   bash showcase/setup.sh
#
# The demo content lives in the repo now — showcase/Demo (Obsidian vault), showcase/demo-go,
# showcase/demo-java, showcase/demo-go.code-workspace — so "pristine" means "as in git":
# tracked files are restored, stray untracked files inside them are removed, and ignored
# editor state (.idea/, Obsidian's workspace.json, the vault's plugin copy) is left alone.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/.." && pwd)"

mkdir -p "$HERE/run"

cd "$REPO"
git checkout -q -- showcase/Demo showcase/demo-go showcase/demo-java showcase/demo-go.code-workspace 2>/dev/null \
  || echo "note: git checkout refused (not a checkout, or sandboxed); content left as is"
git clean -qfd -- showcase/Demo showcase/demo-go showcase/demo-java 2>/dev/null || true

echo "showcase content reset to the committed state"
cat <<'EOF'

One-time GUI steps (if not done yet):
  Obsidian : File → Open vault → Open folder as vault → showcase/Demo; Settings → Community plugins →
             Restricted mode OFF; Settings → Editor → Vim key bindings ON; QUIT Obsidian, run
             `zmk-vim-mode install --obsidian`, reopen; Settings → Appearance → Zoom 125%.
             Close every other vault window before recording.
  IntelliJ : File → Open → showcase/demo-java; accept the JDK prompt (download if needed);
             View → Appearance → Zoom IDE In (~125%); Settings → Editor → Font 20 (revert after).
  VS Code  : `code showcase/demo-go.code-workspace` once, trust the folder.
  Neovim   : open the demo terminal (the command is in env/ghostty-demo.conf; it is Bash with
             env/demo.bashrc, starting in showcase/demo-go), then `nvim`.
EOF
