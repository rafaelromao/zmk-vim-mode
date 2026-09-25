# Bar widget (Omarchy Quattro)

A plugin for Omarchy 4's shell, `rafaelromao.zmk-vim-mode`, that shows the vim
mode your keyboard is in: the Neovim glyph and NORMAL, INSERT, VISUAL, CMDLINE,
VIM or RAW, `VIM ?` while the daemon is down, and no tile at all while vim mode
is off. Once a second it runs `zmk-vim-mode status --bar`, the same answer the
macOS menu bar item draws, and renders it; the wording lives in the daemon, not
here.

## Install

`make install` does it on Omarchy; `zmk-vim-mode install --omarchy` does it
alone. It:

- writes the plugin to `~/.config/omarchy/plugins/rafaelromao.zmk-vim-mode/`
  as regular files, with the path of the binary that installed it written in:
  the shell does not start commands through your login shell. Omarchy's plugin
  validator refuses symlinks inside a plugin, so the files are always real
  copies. The install runs `omarchy plugin validate` on it and
  `omarchy-shell shell rescanPlugins`;
- lists `{"id": "rafaelromao.zmk-vim-mode"}` first in `bar.layout.right` of
  `~/.config/omarchy/shell.json`. Being in the layout is what enables a
  third-party widget, so an entry you already placed anywhere is left where it
  is. The edit is textual and in place: the rest of the file keeps its
  formatting and key order, a `shell.json` that is a symlink stays one, and
  the original is kept as `shell.json.bak-zmk-vim-mode`.

Without a `shell.json` of your own it prints
`omarchy plugin enable rafaelromao.zmk-vim-mode` rather than write one: the
shell does not merge a partial file into its default layout, and a file with
only this widget in it would take every other widget away.

After an update, run `omarchy restart shell` if the bar still shows the old
widget; the shell caches QML it has loaded. `zmk-vim-mode doctor` checks that
the plugin is there and in the bar.

## Colours

The widget draws plain text in the bar's own style; the glyph is the first
word. A bar that colours its widgets' icons by id can match
`rafaelromao.zmk-vim-mode`.

## By hand

Copy `rafaelromao.zmk-vim-mode/` into `~/.config/omarchy/plugins/`, then:

```bash
omarchy-shell shell rescanPlugins
omarchy plugin enable rafaelromao.zmk-vim-mode
```

A copy that was not installed runs `~/.local/bin/zmk-vim-mode`, where
`make install` puts it.
