# zmk-vim-mode for VSCode

The keyboard follows VSCode as closely as it follows Neovim: the exact vim mode
in the editor, `raw` (plain base layout) everywhere else -- terminal, sidebar,
panels, quick inputs, rename box, non-text editors -- and back to the mode the
instant focus returns to the text.

## Install

```bash
zmk-vim-mode install --vscode --atspi && systemctl --user restart zmk-vim-mode
```

Then quit VSCode fully and start it again. Add `vscode = true` to your lazy.nvim
spec for this plugin if it predates that line (see *Layer 1* below). Check with
`zmk-vim-mode doctor`.

What the command does, all idempotent and each with a backup:

- `settings.json` (every VSCode flavour found): `window.title` gets the
  ` [${focusedView}]` marker appended to whatever template you have;
  `editor.accessibilitySupport` is set to `off`.
- `~/.config/code-flags.conf`: `--force-renderer-accessibility` (only with
  `--atspi`; Arch's `code` wrapper appends the file's lines to the command line).
- Installs vscode-neovim if missing and the companion extension, packaged from
  files embedded in the daemon binary -- no node, no vsce, no network for it.
  It never uninstalls anything: if VSCodeVim is present it prints the command.
- Writes the service unit with `--atspi`, which stays on across later
  reinstalls.

## How it works: four layers

Each layer covers what the previous ones cannot see. Inside VSCode the daemon
ranks them: window title → accessibility bus → companion → embedded Neovim →
`legacy`.

### Layer 1 -- the mode: Neovim inside VSCode

[vscode-neovim](https://github.com/vscode-neovim/vscode-neovim) embeds a real
Neovim, which loads your config, which loads the zmk-vim-mode Neovim plugin,
which reports the exact mode to the daemon. Nothing VSCode-specific is
involved. VSCodeVim cannot do this (it reports no modes) and conflicts with
vscode-neovim: remove it.

The spec must stay enabled inside VSCode. LazyVim loads its `vscode` extra
automatically there and disables every plugin except a whitelist and those
marked `vscode = true`; `contrib/nvim-lazy-spec.lua` sets it. Other configs:
no `cond = not vim.g.vscode` around it.

The plugin also watches vscode-neovim's outgoing calls: a mapping that opens a
quick input or the rename box (LazyVim's `<leader><space>`, `<leader>ss`,
`<leader>cr`, anything through `require("vscode").action` matching
`vscode_raw_actions`) raises `raw` until keys reach Neovim again, the companion
or the bus reports the close, or `vscode_raw_ttl_ms` (20 s).

### Layer 2 -- tool windows: the window title

No extension API fires when focus moves to the terminal, the sidebar or a
panel. The title can carry it: with the marker installed above,
`${focusedView}` reads `Text Editor` while the editor has focus, the view's
name or id inside a view (`[terminal]`, `[Explorer]`, `[Search]`), and nothing
at all in widgets outside any view (the Extensions search box). The daemon
follows Hyprland's title events and switches to `raw` whenever the brackets
hold anything but an editor name -- an empty marker included.
`hyprctl activewindow -j | grep title` shows what VSCode publishes.

### Layer 3 -- the companion extension

Plain JavaScript, no build step (`editors/vscode/`). It reports `none` ("no
opinion") while a text editor has focus and `raw`:

- while no text editor is active: Settings, Extensions view, webviews,
  images, an empty window;
- after it opened a quick input or the rename box itself, through the
  keybindings it contributes for `Ctrl+P`, `Ctrl+Shift+P`, `F1`, `Ctrl+G`,
  `Ctrl+Shift+O` and `F2` (active only while the editor has focus);
- after the Neovim plugin forwarded a mapping-opened quick input to it.

It clears the hint on `Escape` inside a quick input or the rename box, when
the active editor or the cursor changes, when the window loses focus, when
the daemon reports editor focus (layer 4), or after `zmkVimMode.quickInputTtlMs`.

### Layer 4 -- everything opened with the mouse: the accessibility bus

Clicking the title-bar Command Center, a breadcrumb or a status-bar picker is
invisible to layers 1-3. The AT-SPI2 bus sees every focus change once
accessibility is on. The daemon (`--atspi`) registers as a listener, joins
events to the frontmost window by pid, and classifies the focused widget:
Monaco's edit surface (class `native-edit-context` / `inputarea`,
`aria-roledescription` "editor") → the clients decide; anything else → `raw`.
It also sends `editor_focus` to the companion and the embedded Neovim when the
editor regains focus, so their hints clear at once -- no mis-typed first key.

Requirements, all handled by the installer: both `org.a11y.Status` flags on
the session (`IsEnabled`, `ScreenReaderEnabled`), and VSCode started with
`--force-renderer-accessibility` -- without it Electron joins the bus but
exposes none of its DOM. Cost and privacy notes are in the main README.

## Verify

```bash
zmk-vim-mode doctor            # title marker, extensions, lazy spec, bus flags, renderer flag
zmk-vim-mode status            # clients: nvim app=vscode + vscode app=vscode; focus line with --atspi
zmk-vim-mode atspi-watch       # one JSON line per focus event, with the classifier's verdict
```

Expected `status` reasons: `client vscode` in the editor, `tool window
focused: terminal` in the terminal, `focus in list item…` or `focus in
input.input…` with a quick input open (via the bus), `vscode client raw` for a
hint without the bus.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `status` says `legacy app code` | no `nvim app=vscode` client: plugin disabled inside VSCode | `vscode = true` in the spec; restart VSCode; `:ZmkVimMode status` via *Neovim: Run command* |
| terminal / sidebar keep the vim layers | title marker missing or VSCode not restarted | `install --vscode`, restart VSCode, check `hyprctl activewindow -j` |
| mouse-opened palette keeps the vim layers | bus off, or renderer flag missing | `install --vscode --atspi`, restart daemon and VSCode; `atspi-watch` must list `code` and print events |
| a widget gets the wrong verdict | classifier does not know it | paste the `atspi-watch` line; rules live in `internal/focus/atspi/classify.go` |
| `doctor`: daemon version differs from CLI | `make install` replaced the binary, not the process | `systemctl --user restart zmk-vim-mode` |
| companion command missing from the palette | extension not installed/loaded | `install --vscode` again, *Developer: Show Running Extensions* |

## Limits

- Without `--atspi`: mouse-opened quick inputs raise no hint; the first key
  after `Escape` from a hinted quick input may go through the base layout when
  no other close signal fired. Letter chords such as `Ctrl+Shift+P` cannot be
  typed while the keyboard is in its NORMAL layer (letters are remapped): use
  `F1` or mappings.
- The find widget's own input is a Monaco editor; the bus tells it apart by its
  label ("Find"/"Replace"). With vscode-neovim, `/` search is Neovim's and
  reports `cmdline`.
- Labels are matched in English; a localized UI may need the widget names in
  `classify.go` extended (the `atspi-watch` output shows them).

Settings: `zmkVimMode.socket`, `zmkVimMode.quickInputTtlMs`, `zmkVimMode.debug`
(logs to the *ZMK Vim Mode* output channel). Commands: *ZMK Vim Mode: Status*
and the wrapped quick-input commands.
