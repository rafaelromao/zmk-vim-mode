# Rafael Romão's keymap and supporting tools — video script

**Format:** screen recording with English voice-over, 7:00 target (6–8 min).
**Resolution:** recorded at 2560×1440, delivered at 1920×1080 (see `env/obs-scene.md`).
**On screen throughout the demos:** the editor, the typed-keys strip (bottom-left) and the
layer HUD (top-right, the Diamond's active layer with keys and combos lighting up).
**Narration pace:** ~140 words per minute; each beat lists its word count.

The keymap on screen is the one in [rafaelromao/keyboards](https://github.com/rafaelromao/keyboards);
the tool is [rafaelromao/zmk-vim-mode](https://github.com/rafaelromao/zmk-vim-mode); the layout is
[Romak](https://rafaelromao.github.io/romak).

---

## The argument in one paragraph

Alternative layouts scatter vim's keys, remapping vim is the wrong fix, and a keyboard that
*guesses* the editor's mode drops keystrokes. zmk-vim-mode makes the editor tell the keyboard
its exact mode over a channel every keyboard already has — three unused LED indicator bits —
so the Diamond can keep a dedicated command layout for NORMAL mode and Romak, untouched, for
INSERT. It follows Neovim, VS Code, IntelliJ and Obsidian, steps aside (RAW) wherever letters
are commands, and installs with one command.

---

## Beats

### 0 · Cold open — 0:00–0:25 (58 words)

**Screen.** Ghostty, Neovim with `internal/modes/modes.go` open, INSERT mode, cursor in a
comment. HUD shows `alpha1` with the INSERT chip. Type a few words, then `Esc`, then `h j k l`
slowly, twice.

**Expect.** HUD: INSERT → NORMAL on `Esc`; on the vim layer the right home row lights
index→pinky in order. Before `Esc`, while INSERT, tap `h` `j` `k` `l` once too: `h` lights the
magic key, `j` lights the `h,` combo, `k` lights the `mg` combo, `l` lights the top row.

**Narration.**
> This is a twenty-four-key keyboard running a layout called Romak. Watch where vim's h, j, k
> and l live while I'm typing: h is a magic key, j and k are two-key combos, l is up on the top
> row. Now I press Escape. The keyboard changed layers with the editor. h j k l are on the home
> row. Nothing in vim was remapped.

### 1 · Title — 0:25–0:45 (46 words)

**Screen.** Title card: *Rafael Romão's keymap and supporting tools*. Cut to `Diamond.jpeg`,
then `overview.png` (from the keyboards repo), then the three logos/names: Diamond · Magic
Romak · zmk-vim-mode.

**Narration.**
> I'm Rafael Romão. This is my keyboard, the Diamond; my layout, Magic Romak; and
> zmk-vim-mode, the tool that keeps the keyboard's layers in sync with the editor's vim state.
> Today it follows Neovim, VS Code, IntelliJ IDEA and Obsidian. Let me show you why it exists.

### 2 · The problem — 0:45–1:40 (128 words)

**Screen.** `alpha1.png` full frame; highlight `h|v`, the `h,`/`mg` combos and `l`. Then
`vim.png`: highlight the right home row `h j k l`, the left-hand operators, the `x d y p v`
combos. Short cut to the old approach: a `.dtsi` excerpt with `&none` positions (optional).

**Narration.**
> Every alternative layout does this to vim. Romak puts h on a magic key that types v after a
> vowel, j and k are combos, l is a top-row reach. Colemak, Dvorak and Gallium each scatter
> them differently. The usual fixes are all bad: remap vim, and you fight every plugin, every
> tutorial and every machine you ssh into. Learn the scattered positions, and your best layout
> is worst at the thing you do most.
>
> A vim layer removes the compromise. In normal mode the keyboard isn't typing letters, it's
> issuing commands, so that layer can be a different map: motions on the home row, operators
> under the other hand, dd and yy as single keys. The catch is knowing *when* you're in normal
> mode. My keyboard used to guess by watching keys, and a wrong guess drops keystrokes.

### 3 · How it works — 1:40–2:30 (122 words)

**Screen.** README diagram of the HID report descriptor (`05 08 19 01 29 05…`), then the
code table (0 off, 1 normal, 2 insert, 3 visual, 4 legacy, 5 cmdline, 6 raw, 7 legacy silent).
Terminal: `zmk-vim-mode status`; then `zmk-vim-mode set insert`, `set normal`, `set off`,
`set off` again (back to auto) — the HUD chip follows each one; the daemon log's
`led write … code=N` lines scroll in a second pane (`tail -f ~/Library/Logs/zmk-vim-mode.log | grep -E 'decision|led'`).

**Expect.** `status`: `decision : normal (code 1) — nvim client`, the Diamond listed
`writable`. Each `set` produces one `led write` line per device.

**Narration.**
> The editor knows its mode, so the editor should say it. Three parts: a Neovim plugin, a
> host daemon, and a ZMK module. The channel is one every keyboard already has: the LED
> report the OS uses for Caps Lock and Num Lock. Five indicator bits; three of them — Compose,
> Kana and Scroll Lock — no operating system ever sets. Read as a number, that's eight codes,
> over USB or Bluetooth, with stock ZMK and no pairing.
>
> The daemon decides the code from the focused window and from the plugins reporting in,
> writes it to every endpoint the keyboard exposes, and the firmware flips a layer bitmask.
> The keyboard still infers locally between keystrokes, so it never waits on the host; the
> host corrects it within a millisecond.

### 4 · Neovim — 2:30–3:40 (150 words)

**Screen.** Ghostty (demo shell) in `showcase/demo-go`. Actions, each with the HUD/`status` expectation:

| Action | HUD chip | `status` reason |
|---|---|---|
| `nvim` → LazyVim dashboard | RAW | `nvim client` (mode raw) |
| `f` → pick `modes.go`, Enter | NORMAL | `nvim client` |
| **the tour**, slowly: `j j j k`, `l l l h h`, `w w e b b`, `0 $ 0`, `i` `Esc`, `a` `Esc` — every one on the right home row or its neighbours | NORMAL, INSERT, NORMAL | the HUD lights `h j k l` index→pinky |
| `/Compose` Enter (CMDLINE on the way), `A`, type ` // bit 0 of the code`, `Esc`, `u` | CMDLINE → NORMAL → INSERT → NORMAL | |
| `v`, `j`, `j`, `y`, `Esc` | VISUAL → NORMAL | |
| `:` then `%s/leds/report/g` Enter | CMDLINE → NORMAL | |
| `<space>` (leader) | RAW for ~0.3 s, until which-key opens, then NORMAL | `nvim client` (mode raw, briefly) |
| `ff` → picker opens, type `readme` (r would be *replace* on the vim layer), `Esc` `Esc` | INSERT (the picker's input) → NORMAL | |
| `<space>e` → explorer, `j` `j` `k` move in the tree, `<space>e` back | RAW → NORMAL | |
| `:terminal` then `i`, type `go test ./...`, Enter, `Ctrl-\ Ctrl-n`, `:bd!` | RAW → NORMAL | |
| `<space>l` → Lazy, `q` | RAW → NORMAL | |

**Narration.**
> Neovim first, because the plugin reports the real mode from inside the editor. The
> dashboard: every letter here is a command, so the keyboard reports *raw* and shows my plain
> layout. Open a file: normal. i: insert — Romak is back, untouched. Escape: normal again.
> Visual, yank, escape. Colon opens the command line on its own layer.
>
> Now the part no window-title trick could do. Space is my leader: the moment I press it the
> keyboard goes raw, because the next letter is a menu shortcut. A picker is a text field: insert. The file explorer:
> raw — a, d and r reach the tree, not vim. The terminal: raw, so Escape belongs to the shell.
> Lazy: raw. Back in the file: normal. The keyboard tracks what the editor is actually
> expecting, not what window is in front.

### 5 · VS Code — 3:40–4:35 (118 words)

**Screen.** `code showcase/demo-go.code-workspace`, open `modes.go`. Title bar reads
`modes.go — demo-go [Text Editor]`.

| Action | HUD chip | `status` reason |
|---|---|---|
| click into the editor, `Esc`, then the same tour (`hjkl`, `web`, `i`/`a`) | NORMAL | `client vscode` |
| `/Kana` Enter, `A`, type ` // bit 1 of the code`, `Esc`; `v`, `Esc`; `u` at the end | INSERT/VISUAL/NORMAL | |
| `` Ctrl+` `` → terminal, type `go run ./cmd/vimmode`, Enter | RAW | `tool window focused: Terminal` |
| `` Ctrl+` `` → back to the editor | NORMAL | `client vscode` |
| `F1` → command palette, type `keyboard` (k would be *up*), `Esc` | RAW → NORMAL | `tool window focused: widget outside any view` |
| `⇧⌘E` → Explorer sidebar, `↓` `↓` (letters create files in this Explorer setup), `⇧⌘E` again → back to the editor | RAW → NORMAL | `tool window focused: Folders` |

**Narration.**
> VS Code runs a real Neovim inside it, through vscode-neovim, and that Neovim loads the same
> plugin. So the modes are exact, not approximated. Where VS Code differs is everything around
> the editor. The terminal is a tool window, and no extension API fires when focus moves there;
> the daemon reads it from the window title, which now carries the focused view. Raw. Back in
> the editor: normal. The command palette is a quick input, and a small companion extension
> reports it: raw while it's open, mode back the instant it closes. The sidebar: raw. Four
> layers of detection, ranked by the daemon, and the keyboard just sees a code.

### 6 · IntelliJ IDEA — 4:35–5:25 (110 words)

**Screen.** IntelliJ, `demo-java`, `ModeTable.java` open, IdeaVim on.

| Action | HUD chip | `status` reason |
|---|---|---|
| open `ModeTable.java` (`⇧⌘O`) — already normal mode, no `Esc` (IdeaVim beeps on `Esc` in normal mode; `set visualbell` in `~/.ideavimrc` silences it), then the tour (`hjkl`, `web`, `i`/`a`) | NORMAL | `client intellij` |
| `/COMPOSE` Enter, `A`, type ` // Compose is bit 0`, `Esc`; `v` `e` `y` `Esc`; `:` `w` Enter | INSERT/VISUAL → NORMAL; `:` shows **RAW** (IdeaVim's ex line is a separate component, so the plugin reports focus left the editor) | `intellij client raw` while the ex line is open |
| Meh+B → Project tool window (the MEHS layer's *project* key), type `readme` in the speed search, `Esc` `Esc` | RAW → NORMAL | `intellij client raw` |
| `Esc` → back to the editor | NORMAL | |
| `⌥F12` → terminal, type `ls`, Enter; `Esc`/`⌥F12` back | RAW → NORMAL | |
| Meh+G (`Ctrl+Alt+Shift+G`) → *Reformat Code* | — | shown as held modifiers on the HUD |

**Narration.**
> IntelliJ has IdeaVim, and IdeaVim has a mode listener, so this plugin subscribes to it and
> reports the same four modes. Insert, visual, the command line — same layers as Neovim.
>
> The project tree: raw. IntelliJ's terminal and consoles are technically editors, so the
> plugin tells them apart by their kind and reports raw there too. Without the plugin IntelliJ
> still works — the daemon falls back to legacy mode and the keyboard infers by itself, the way
> it did for years. And this is the MEHS layer: my IDE actions are Meh and Hyper chords the
> firmware emits and a keymap in the repo binds. Format code, one key.

### 7 · Obsidian — 5:25–6:10 (100 words)

**Screen.** Obsidian, the *Demo* vault (`showcase/Demo`), note *Tasks*.

| Action | HUD chip | `status` reason |
|---|---|---|
| click into the note, `Esc`, then the tour (`hjkl`, `web`, `i`/`a`) | NORMAL | `client obsidian` |
| `j` to a task, `i`, type, `Esc` | INSERT → NORMAL | |
| `o` then the `- [ ]` macro key, type `Record the wrap-up`, `Esc` | INSERT → NORMAL | |
| `:` → `w` Enter | CMDLINE → NORMAL | |
| `⌘E` → reading view | RAW | `client obsidian` (mode raw) |
| `⌘E` back; click the note title | NORMAL → RAW | |
| `⇧⌘F` → search pane, type `layer`, then click back into the note body | RAW → NORMAL | `obsidian client raw` while searching |

**Narration.**
> Obsidian's editor is CodeMirror with a vim extension, and its own plugin listens to it:
> normal, insert, the colon dialog as the command line. My macros layer has a key for a new
> task, so a checklist item is one press. Reading view has no vim, so the keyboard reports raw
> and Romak is back. The note title, the sidebar, search: raw. Focus returns to the text:
> normal. Four editors, four different plumbing jobs, one daemon, and the keyboard never has to
> guess which one it's talking to.

### 8 · Everywhere else — 6:10–6:35 (58 words)

**Screen.** `⌘Tab` to the plain Ghostty shell (no Neovim running) — HUD: OFF, reason
`terminal without nvim client`; any non-editor app gives OFF too (`non-editor app <bundle id>`).
Back to Ghostty (no nvim) — OFF. In Ghostty: `zmk-vim-mode set raw`, HUD RAW with `override`;
`zmk-vim-mode set raw` again — back to auto. `zmk-vim-mode devices` shows the Diamond on USB.

**Narration.**
> Anywhere else the vim layers are simply off. For the cases nothing can detect — vim over
> ssh, a screen-sharing app — set is the escape hatch, and repeating it returns to automatic.
> The keyboard listens on USB and Bluetooth at once, so the same code reaches it whichever
> host it's paired to.

### 9 · Install and wrap-up — 6:35–7:00 (62 words)

**Screen.** Pre-rendered terminal (not live; `doctor` prints home paths): `make install`
summary lines, then `zmk-vim-mode doctor` with every check green. End card with the three
links.

**Narration.**
> One command installs the daemon, the service, the Neovim spec and the editor extensions, and
> doctor tells you what's left — on macOS, two permissions. The keymap, the layout and the tool
> are all on my GitHub. If you type on an alternative layout and live in vim, this is the
> compromise you no longer have to make.

---

## Shot list

| # | Shot | Source | Notes |
|---|---|---|---|
| A | Diamond photo | `keyboards/docs/img/builds/Diamond.jpeg` | title beat |
| B | Layer diagrams | `keyboards/docs/img/diagrams/{overview,alpha1,vim,numbers,symbols}.png` | beats 1–2; zoom on regions |
| C | HID descriptor + code table | README.md of zmk-vim-mode (render as text cards) | beat 3 |
| D | Terminal: `status`, `set`, log tail | live, demo shell | beat 3 |
| E | Neovim segment | live | beat 4 |
| F | VS Code segment | live | beat 5 |
| G | IntelliJ segment | live | beat 6 |
| H | Obsidian segment | live | beat 7 |
| I | Safari + `set` | live | beat 8 |
| J | `make install` / `doctor` | pre-rendered or blurred paths | beat 9 |
| K | B-roll: hands on the Diamond | phone camera, top-down, 30 s | cut under beats 0, 2 and 4 |
| L | End card | title + three URLs | beat 9 |

## Pass/fail per take

Run `zmk-vim-mode status` at the end of each segment; the *reason* column above is the
expected value. A take with a different reason is a setup problem (plugin not loaded, title
marker missing, Accessibility not granted), not a script problem — see the editor READMEs'
troubleshooting tables.

## Words → time

| Beat | Words | Time |
|---|---|---|
| 0 | 58 | 0:25 |
| 1 | 46 | 0:20 |
| 2 | 128 | 0:55 |
| 3 | 122 | 0:50 |
| 4 | 150 | 1:10 |
| 5 | 118 | 0:55 |
| 6 | 110 | 0:50 |
| 7 | 100 | 0:45 |
| 8 | 58 | 0:25 |
| 9 | 62 | 0:25 |
| **total** | **952** | **7:00** |

Narration is slower than the actions in beats 4–7; pause the voice, let the HUD flip, then
continue. Cut the actions, not the words.
