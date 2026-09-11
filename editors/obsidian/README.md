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
listed in Obsidian's `obsidian.json` and, **when Obsidian is not running**,
adds the plugin to each vault's `community-plugins.json` (backup kept).

Obsidian owns that file and rewrites it from memory when it quits, so an entry
added under a running Obsidian is silently discarded — the plugin then sits in
the vault, unlisted and never loaded. Either quit Obsidian before running the
command, or enable *ZMK Vim Mode* yourself: Settings → Community plugins →
reload → toggle it on.

Restricted mode must be off (Settings → Community plugins) and Vim key
bindings on (Settings → Editor).

By hand, equivalently:

```bash
mkdir -p "<vault>/.obsidian/plugins/zmk-vim-mode"
cp editors/obsidian/manifest.json editors/obsidian/main.js "<vault>/.obsidian/plugins/zmk-vim-mode/"
```

then enable *ZMK Vim Mode* under Community plugins.

## Verify

```bash
zmk-vim-mode doctor            # plugin installed and enabled, per vault
zmk-vim-mode status            # a client "obsidian app=obsidian"; reason "client obsidian"
```

In Obsidian, palette → *ZMK Vim Mode: Status* shows the connection and the
state being reported right now. Then: `i` → INSERT layer, `Esc` → NORMAL,
`v` → VISUAL, `:` → CMDLINE, click the note title, a sidebar, or switch to
reading view → base layout.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| no `obsidian` client in `status` at all | the plugin is not loaded | is it listed and on under Settings → Community plugins? if it is missing, reload the list there (see *Install*) |
| `status` says `legacy app md.obsidian.Obsidian` | plugin loaded but not connected | *ZMK Vim Mode: Status* in the palette; daemon running? |
| *Status* says not connected | socket unreachable | daemon running? Flatpak Obsidian cannot see `~/.local/state`; use the native package |
| every note reports `raw` | editor adapter shape unknown to the plugin | Obsidian version? Ctrl+Shift+I console, filter `zmk`; see *How it finds the editor* |
| vim modes not reported at all | Vim key bindings off | Settings → Editor → Vim key bindings |

## Limits

- Non-markdown views (Canvas, graph, PDF) report `raw`: the keyboard is in its
  base layout there, which is what those views expect.
- The `:`/`/` dialog is recognised by focus landing inside the editor's
  container but outside the text; a theme that moves the dialog elsewhere
  would make it `raw` instead of `cmdline`.
- Operator-pending states (`d`, `c` awaiting a motion) are not distinct in
  codemirror-vim's events; the keyboard's own CHANGE layer handles them.

## How it finds the editor

The adapter is looked up as `view.editMode.editor.cm.cm`, falling back to
`view.editor.cm.cm` and the pre-1.0 `view.sourceMode.cmEditor.cm.cm` -- the
same path the popular *Vimrc Support* plugin uses. Listeners are (re)attached
on `active-leaf-change`, `file-open` and `layout-change`, and a `focusin`
listener per window (pop-outs included) re-evaluates focus.

If a future Obsidian changes the shape, the plugin degrades to `raw` for every
markdown view rather than guessing; *Status* will say so.
