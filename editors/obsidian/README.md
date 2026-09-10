# zmk-vim-mode for Obsidian

Reports the vim mode of Obsidian's editor (CodeMirror + `@replit/codemirror-vim`)
and where keyboard focus is, so the keyboard follows Obsidian as closely as it
follows Neovim.

| Obsidian state                                   | reported |
|--------------------------------------------------|----------|
| text focused, vim normal / insert / visual       | that mode (replace → insert) |
| the vim `:` or `/` dialog focused                | cmdline  |
| reading view, no markdown view, vim keys off     | raw      |
| focus outside the text: sidebar, search, settings, note title, properties | raw |

## Install

```bash
zmk-vim-mode install --obsidian
```

copies the two files (embedded in the binary; no build step) into every vault
listed in Obsidian's `obsidian.json` and adds the plugin to each vault's
`community-plugins.json` (backup kept). Restart Obsidian. Restricted mode must
be off (Settings → Community plugins) and Vim key bindings on (Settings →
Editor).

By hand, equivalently:

```bash
mkdir -p "<vault>/.obsidian/plugins/zmk-vim-mode"
cp editors/obsidian/manifest.json editors/obsidian/main.js "<vault>/.obsidian/plugins/zmk-vim-mode/"
```

then enable *ZMK Vim Mode* under Community plugins.

`zmk-vim-mode status` then shows a client `obsidian app=obsidian`. The command
*ZMK Vim Mode: Status* shows the connection and the state being reported.

## How it finds the editor

The adapter is looked up as `view.editMode.editor.cm.cm`, falling back to
`view.editor.cm.cm` and the pre-1.0 `view.sourceMode.cmEditor.cm.cm` -- the
same path the popular *Vimrc Support* plugin uses. Listeners are (re)attached
on `active-leaf-change`, `file-open` and `layout-change`, and a `focusin`
listener per window (pop-outs included) re-evaluates focus.

If a future Obsidian changes the shape, the plugin degrades to `raw` for every
markdown view rather than guessing; *Status* will say so.
