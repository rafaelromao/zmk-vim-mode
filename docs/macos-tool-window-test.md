# macOS tool-window test: handoff

You are testing, on a Mac, that the zmk-vim-mode daemon picks the right mode
for every tool window of Neovim, VSCode, Obsidian and IntelliJ. The same
matrix passed on Linux (Omarchy, Hyprland) on 2026-10-31 at main `01eaa27`.
The Linux verdicts below are the **expected** column. Your job is to find
every place where macOS **diverges** from them, then write a fix plan.

**Done when:** every row of the four matrices has an observed verdict from a
live probe, every divergence is either explained as correct for macOS or has
an entry in the fix plan, and the Mac is back in the state you found it in.
Fix nothing until the user approves the plan.

## Ground rules

- **Restore what you change.** Before testing, save the `override` line of
  `zmk-vim-mode status`. Tests need no override, so clear it with
  `zmk-vim-mode set auto`. When you finish, put it back; the user usually runs
  `zmk-vim-mode set off --sticky --no-toggle`. A daemon restart drops the
  override, so check it again after every `make install` or `restart`.
- **Edit scratch files only.** Synthetic keys land in whichever window has
  focus, and a fuzzy palette match can run the wrong command. Open each editor
  on a scratch file or project outside the repo. Before pressing Return in a
  palette, take a screenshot and check which entry is highlighted.
- **Guard every keystroke.** Before each key, activate the target app and
  check that it really is frontmost (`status --json` → `.frontmost.class`). If
  it isn't, stop.

## Modes, in one line each

normal / insert / visual / cmdline follow the editor's vim mode. **raw** means
keys must reach the app untouched: a picker, a tree, a terminal, a dialog, a
settings page. **off** is a non-editor app. The reason string that
`status --json` prints next to each verdict says which rule fired. Read it,
because two rules can give the same mode for different reasons.

## 1. Update and install

1. `git checkout main && git pull --ff-only origin main`. Expect `01eaa27` or
   later.
2. `make install`. On the first run it may ask for the login password to
   create the signing certificate.
3. Grants: under System Settings → Privacy & Security, both **Input
   Monitoring** and **Accessibility** must list `~/.local/bin/zmk-vim-mode`.
   Without Accessibility the daemon sees no window titles, and every
   title-based row below fails for that reason alone. Only the user can grant
   them; if a grant is missing, ask them. Then run `zmk-vim-mode restart`.
4. `zmk-vim-mode doctor`. It should report 0 failed. The accessibility-bus
   check is Linux-only.
5. Neovim takes the plugin from lazy.nvim, not from `make install`, so run
   `nvim --headless "+Lazy! update zmk-vim-mode" +qa`. Check that
   `git -C ~/.local/share/nvim/lazy/zmk-vim-mode log -1 --oneline` matches the
   repo.
6. IntelliJ: check that each IDE's `plugins/zmk-vim-mode-intellij/lib` holds
   **one** jar. On macOS the plugins directory is under
   `~/Library/Application Support/JetBrains/<IDE>/plugins`.
7. Restart every editor that was already running. Plugins load at startup,
   and IntelliJ's client stays disconnected after a daemon restart until the
   IDE's mode changes (a known issue).
8. `zmk-vim-mode status`. Expect one client each for `nvim`, `vscode` (plus
   VSCode's embedded `nvim`), `obsidian` and `intellij`.

## 2. Driving the GUI on macOS

Linux used `ydotool` and Hyprland workspaces. On a Mac:

- **Activate:** `osascript -e 'tell application "Visual Studio Code" to activate'`.
  Use `Obsidian`, `IntelliJ IDEA` (or `IntelliJ IDEA CE`) and `Ghostty`
  for the others.
- **Keys:** System Events is the tool, and the app running your shell needs
  the Accessibility grant for it to work.
  - Text: `osascript -e 'tell application "System Events" to keystroke "i"'`.
  - Chords: `keystroke "p" using {command down, shift down}`.
  - Named keys by key code: Esc 53, Return 36, Tab 48, F2 120, F6 97,
    F12 111. For example `key code 97 using shift down` for Shift+F6.
  - Leave a short pause (0.1–0.3 s) between keys: Electron apps drop keys that
    come back to back.
- **Mouse:** use `cliclick` (Homebrew) if it is installed. Otherwise click
  with System Events, or ask the user to install it.
- **Screenshots:** `screencapture -x out.png`. Read the image before pressing
  Return in any palette.
- **Probe:** after each step, wait for the UI to settle and record the result:

  ```bash
  probe() { printf '%-36s ' "$1"; zmk-vim-mode status --json |
    jq -r '"\(.frontmost.class) | \(.frontmost.title // "") => \(.mode) — \(.reason)"' |
    tee -a results.log; }
  ```

- **Neovim state:** start the test Neovim with
  `--listen "$TMPDIR/zvm-nvim.sock"`. Before each step, close every window
  except the first and run `stopinsert` over RPC. Read back the filetype,
  buftype and mode the same way, so you can tell a wrong verdict from a key
  that never landed:

  ```bash
  nvim --server "$TMPDIR/zvm-nvim.sock" --remote-expr 'luaeval("(function() local b=vim.api.nvim_get_current_buf() return vim.bo[b].filetype..\"/\"..vim.bo[b].buftype..\" mode=\"..vim.api.nvim_get_mode().mode end)()")'
  ```

Run the Neovim test in Ghostty
(`open -na Ghostty --args --title=zvm-nvim-test -e nvim --listen … <file>`)
or in Terminal.app. Both bundle ids count as terminals. Tests start from a
fresh state each time: press Esc twice and click into the editor's text.

## 3. Matrices

The key column gives the Mac binding. In vscode-neovim and IdeaVim, Ctrl
chords go to vim, so app shortcuts use Cmd. If a binding differs on this Mac,
use the menu item instead and note the change.

### Neovim (LazyVim, leader = Space)

| Step | Keys | Expected |
|---|---|---|
| normal | Esc | normal |
| insert | `i` | insert |
| visual | `v` | visual |
| command line | `:` | cmdline |
| search | `/` | cmdline |
| leader pending, probed after 0.1 s | Space | raw |
| which-key menu, probed after 1.2 s | Space | raw |
| files picker, prompt in insert | Space Space | raw |
| picker after Esc (list) | Space Space, Esc | raw |
| grep picker | Space `sg` | raw |
| explorer | Space `e` | raw |
| terminal | Ctrl+/ | raw |
| lazy | Space `l` | raw |
| mason | Space `cm` | raw |
| trouble | Space `xx` | normal |
| help | `:help` Return | normal |
| hover | `K` | normal |

### VSCode (vscode-neovim)

| Step | Keys | Expected (reason) |
|---|---|---|
| normal / insert / visual | Esc, `i`, `v` | that mode (client vscode) |
| command line | `:` | cmdline (client vscode command line) |
| search | `/` | cmdline (client vscode command line) |
| explorer | Cmd+Shift+E | raw (tool window focused: Folders) |
| search view | Cmd+Shift+F | raw (Search) |
| source control | Ctrl+Shift+G | raw (Changes) |
| extensions | Cmd+Shift+X | raw |
| run and debug | Cmd+Shift+D | raw (Run) |
| problems | Cmd+Shift+M | raw (Problems) |
| terminal (focused) | Ctrl+` | raw (Terminal) |
| quick open | Cmd+P | raw |
| command palette | Cmd+Shift+P | raw |
| go to symbol | Cmd+Shift+O | raw |
| settings | Cmd+, | raw |
| **Save As** (untitled file with text) | Cmd+Shift+S | raw |
| **Open File** | Cmd+O | raw |
| **"Save changes?" prompt** | `:q` on the dirty untitled file | raw |

The three bold rows are the ones most likely to diverge. On Linux the file
chooser is a separate xdg-desktop-portal window, matched by `DialogApps`. The
prompt is an untitled window of VSCode's own process, which the "dialog
window" rule catches. On macOS the save panel is a **sheet** of the VSCode
window, and the prompt may be a sheet too. Check the frontmost class and
title the daemon actually sees when each one is open, and record them. The fix
plan depends on them. Answer "Don't Save" to the prompt.

### Obsidian (vim key bindings on; any scratch vault)

| Step | Keys | Expected |
|---|---|---|
| normal / insert / visual | Esc, `i`, `v` | that mode |
| command line / search | `:`, `/` | cmdline |
| quick switcher | Cmd+O | raw |
| command palette | Cmd+P | raw |
| search pane | Cmd+Shift+F | raw |
| find in file | Cmd+F | raw |
| **settings** | Cmd+, | raw |
| settings closed | Esc | normal |
| reading view | Cmd+E | raw |
| back to source | Cmd+E | normal |

On Linux, Settings opens a separate window, and the plugin reports raw when
the editor's document loses focus (`doc.hasFocus()`). Check whether that
holds when the macOS window is laid out differently.

### IntelliJ (IdeaVim; a scratch Java project)

| Step | Keys | Expected |
|---|---|---|
| normal / insert / visual | Esc, `i`, `v` | that mode |
| command line / search | `:`, `/` | cmdline |
| inline rename, caret on a symbol | Shift+F6 | insert |
| after rename | Esc | normal |
| project tree | Cmd+1 | raw |
| terminal | Opt+F12 | raw |
| problems | Cmd+6 | raw |
| structure | Cmd+7 | raw |
| find action | Cmd+Shift+A | raw |
| go to file | Cmd+Shift+O | raw |
| find in files | Cmd+Shift+F | raw |
| settings dialog | Cmd+, | raw |

## 4. Report

When all four matrices are done:

- Restore the override, and close the editors and scratch files you opened.
- Give the user a table per app with step, expected, observed and reason.
  Mark the divergences.
- For each divergence, give the frontmost class and title you saw, the rule
  in `internal/state/decide.go` (or the editor plugin) that decided it, and a
  proposed fix with the test that would cover it. Group the fixes one per
  commit, as was done for PR #2 on Linux.
