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

No build step. Either package it:

```bash
cd editors/vscode
npx @vscode/vsce package
code --install-extension zmk-vim-mode-0.1.0.vsix
```

or symlink it into the extensions folder:

```bash
ln -s "$PWD/editors/vscode" ~/.vscode/extensions/rafaelromao.zmk-vim-mode-0.1.0
```

`zmk-vim-mode status` then shows a second client, `vscode app=vscode`, whose
mode is `none` while the editor has focus and `raw` otherwise.

### Limits

- A quick input is known to close only when the active editor or the cursor
  changes, or the window loses focus; otherwise the hint expires after
  `zmkVimMode.quickInputTtlMs` (20 s). Escaping the palette and pressing a
  motion key immediately sends that first key through the base layout.
- Quick inputs opened from the terminal or the sidebar (editor not focused)
  are not wrapped; the title then says a view has focus, which is already raw.
- The find widget (Ctrl+F) is part of the editor: with vscode-neovim, `/`
  search is Neovim's own and reports `cmdline` correctly.

Settings: `zmkVimMode.socket`, `zmkVimMode.quickInputTtlMs`, `zmkVimMode.debug`
(logs to the *ZMK Vim Mode* output channel). Command: *ZMK Vim Mode: Status*.
