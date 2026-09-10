# zmk-vim-mode for VSCode

Three pieces make VSCode a first-class citizen. Each one is independent; do
them in this order.

## 1. Real modes: switch to vscode-neovim

Install `asvetliakov.vscode-neovim` and uninstall `vscodevim.vim` (they are
mutually exclusive). vscode-neovim embeds a real Neovim, which loads your
config, which loads the zmk-vim-mode Neovim plugin, which reports the exact
mode to the daemon -- nothing VSCode-specific to add.

The spec must stay enabled inside VSCode. LazyVim loads its `vscode` extra
automatically there and disables every plugin except a whitelist and those
marked `vscode = true` -- `contrib/nvim-lazy-spec.lua` sets it; add it to your
copy if it predates this note. Other configs: no `cond = not vim.g.vscode`.

`zmk-vim-mode status` then shows a client `nvim app=vscode`. If it does not,
open the Neovim output channel in VSCode (*Output* → *vscode-neovim*) and run
`:ZmkVimMode status` through the command palette's *Neovim: Run command*.

## 2. Tool windows: publish the focused view in the title

The extension API cannot see focus move to the terminal, the sidebar or a
panel. VSCode's window title can. Add to `settings.json`:

```json
"window.title": "${dirty}${activeEditorShort}${separator}${rootName} [${focusedView}]"
```

`${focusedView}` reads `Text Editor` while the text editor has focus (empty
on older builds) and the view's name or id otherwise (`[terminal]`,
`[Explorer]`, `[Search]`, ...). The daemon reads the title from Hyprland and
switches the keyboard to raw whenever the brackets hold anything but an
editor name (`state.VSCodeEditorViews`). Restart VSCode after changing the
setting; `hyprctl activewindow -j | grep title` shows what it publishes.

## 3. Everything else: this companion extension

Covers what neither nvim nor the title can: no active text editor (Settings,
Extensions, webviews, images, an empty window) and the quick inputs opened
with Ctrl+P, Ctrl+Shift+P, F1, Ctrl+G and Ctrl+Shift+O.

Quick inputs opened **from Neovim mappings** (LazyVim's `<leader><space>`,
`<leader>ss`, `<leader>cr`... anything going through
`require("vscode").action`) are caught on the Neovim side: the plugin watches
vscode-neovim's outgoing calls and raises the same raw hint for the commands in
`vscode_raw_actions` (palette, Go to File/Line/Symbol, rename...). With the
companion installed the two sides share one close detection; without it the
hint clears on the next key that reaches Neovim, or after
`vscode_raw_ttl_ms`. Views and the terminal are not in that list on purpose:
the title reports them, and reports the way back instantly.

## Install: one command

```bash
zmk-vim-mode install --vscode
```

does steps 2 and 3 and the extension half of step 1: it sets `window.title`
(appending the marker to a template you already have) and
`editor.accessibilitySupport: off` in every VSCode flavour's `settings.json`
it finds -- textually, so comments survive, with a `settings.json.bak-zmk-vim-mode`
backup -- packages the companion from files embedded in the binary (no node,
no vsce) and installs it with `code --install-extension --force`, and installs
vscode-neovim if missing. It never uninstalls anything: if VSCodeVim is
present it prints the command for you. Restart VSCode afterwards; run
`zmk-vim-mode doctor` to confirm. The lazy spec (`vscode = true`) stays yours
to add.

Manual alternatives, should you prefer them:

```bash
cd editors/vscode && npx @vscode/vsce package && code --install-extension zmk-vim-mode-0.1.3.vsix
# or
ln -s "$PWD/editors/vscode" ~/.vscode/extensions/rafaelromao.zmk-vim-mode-0.1.3
```

`zmk-vim-mode status` then shows a second client, `vscode app=vscode`, whose
mode is `none` while the editor has focus and `raw` otherwise.

### Limits

- A quick input is known to close on Escape (bound inside quick inputs), when
  the active editor or the cursor changes, or when the window loses focus;
  otherwise the hint expires after `zmkVimMode.quickInputTtlMs` (20 s).
  Accepting an entry that changes nothing visible (a toggle) keeps raw until
  the next cursor move: that first motion key goes through the base layout.
- Selection events in the first 500 ms after opening are ignored: the blur and
  vscode-neovim's resync fire them, and they would clear the hint at once.
- Quick inputs opened from the terminal or the sidebar (editor not focused)
  are not wrapped; the title then says a view has focus, which is already raw.
- **Quick inputs opened with the mouse** -- the title-bar Command Center, a
  breadcrumb, a status-bar item -- raise no hint from this extension: nothing
  in its API observes them and the title does not change. They are covered
  only by the accessibility bus: `zmk-vim-mode install --atspi` (see the main
  README, *Following focus through the accessibility bus*). Without it, open
  them from the keyboard (`F1`, a Neovim mapping) or hide the target with
  `"window.commandCenter": false`. Letter chords such as `Ctrl+Shift+P` are
  not an option while the keyboard sits in its NORMAL layer: the letters are
  remapped there.
- With `--atspi` the daemon also tells this extension and the embedded Neovim
  when focus returns to the editor (`editor_focus`), so their hints clear at
  once and the first key after a quick input is never mis-typed.
- The find widget (Ctrl+F) is part of the editor: with vscode-neovim, `/`
  search is Neovim's own and reports `cmdline` correctly.

Settings: `zmkVimMode.socket`, `zmkVimMode.quickInputTtlMs`, `zmkVimMode.debug`
(logs to the *ZMK Vim Mode* output channel). Command: *ZMK Vim Mode: Status*.
