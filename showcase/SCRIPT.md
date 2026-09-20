# The keyboard that knows your vim mode — video script

**Format:** screen recording plus English narration, nothing else — no camera, no animation, no
music. ~5:15 (4:45–5:30). **Every beat is recorded hands-off** by `record.py`; the narration is
read after the picture is cut, and the automated takes carry a TTS scratch track only for
judging pace.
**Resolution:** the recording monitor at scale 1.25 on 2560×1440 (logical 2048×1152),
delivered at 1920×1080 **60 fps** (`ZMK_RECORD_FPS=60` for `record.py`; see `env/obs-scene.md`).
**On screen throughout the demos:** the editor on the left; on the right rail the layer HUD
([zmk-layer-hud](https://github.com/rafaelromao/zmk-layer-hud)) with the Diamond's active layer and
lit keys, and the typed-keys strip below it. The HUD shows the keyboard's layers; where the
daemon's reason matters (beats 3 and 8 — RAW and OFF both read *Alpha 1*), the segment types
`zmk-vim-mode status` in the demo terminal so the reason itself is on camera.
**Non-demo shots are screen recordings too, driven the same way:** the diagrams are the PNGs
from the keyboards repo opened maximized in the image viewer (`imv` on Omarchy; `ZMK_IMAGE_VIEWER`
overrides it); the title, the "how it works" material, the devicetree node and the end card are
text cards in `env/cards/` printed in the demo terminal with `show <card>`. The physical keyboard
is never on camera, so the narration names the board and the HUD stands in for it.
**Narration pace:** ~140 words per minute; each beat lists its word count.

The keymap on screen is the one in [rafaelromao/keyboards](https://github.com/rafaelromao/keyboards);
the tool is [rafaelromao/zmk-vim-mode](https://github.com/rafaelromao/zmk-vim-mode); the layout is
[Romak](https://rafaelromao.github.io/romak). `YOUTUBE.md` has the title, thumbnail, description
and where to post; `TAKE-2-PLAN.md` is the review of the first assembly and the production plan.

## One process, ten beats

`python3 showcase/record.py all` (or one beat) drives `rehearse.py`'s segment of the same
number for every beat 0–9, with the rehearsal HUD, and lands `run/take<N>.mp4`. The action
tables below are what the segments type, so the take shows exactly this. The demo segments run
longer than the narration on purpose (they exercise every state the rehearsal checks): the
*Cut* line under each beat says which stretch to keep. Before the run: monitor at scale 1.25,
bar widgets stripped (`bash showcase/prepare.sh` does it), the take HUD stopped
(`bash showcase/hud.sh stop`), `bash showcase/prepare.sh` done. The driver parks on
workspace 8 before every take and the segments never leave workspaces 5–8, so nothing
below workspace 5 is ever on camera. `ZMK_RECORD_FPS=60` for the deliverable.

**Typing in every take:** ~70 words per minute — about one character every 0.17 s —
so each key lights on its own and no two fall inside the 30 ms combo window.
Digit strings come off the NUM layer and arrow keys off the nav layer, held for
the whole sequence; neither is ever a combo. (Digits on the vim layer itself —
`0`, `$` in the tours — are single keys and need no hold.) `rehearse.py` types at
this pace.

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

### 0 · Cold open — 0:00–0:18 (57 words) · `record.py 0`

**Screen (`seg0`).** `nvim internal/modes/modes.go` in the demo shell, `:21` (the comment on the
three indicator bits), `A` → INSERT, the HUD reads *Vim insert*; type ` no OS ever sets these`,
pause, `Esc`. Then `j j k`, `l l h`, at the take pace; `u`, `:qa!`, `exit`. No title first: the
flip is the opening shot.

**Cut.** From the first typed letter to the last `h`; drop the file opening and the exit.

**Expect.** HUD *Vim insert* → *Vim normal* on `Esc`; on the vim layer the right home row
lights index → pinky.

**Narration.**
> Watch the panel on the right: that's my keyboard, twenty-four keys, drawn live from its own
> reports, on a layout called Romak. I press Escape — and the keyboard switches layers, because
> the editor just told it vim is in normal mode. h, j, k, l are under my right hand now. Nothing
> in vim was remapped.

### 1 · Title and intro — 0:18–0:37 (45 words) · `record.py 1`

**Screen (`seg1`).** `show title` in the demo terminal (the card in `env/cards/title.txt`),
4 s; `show intro` (the card in `env/cards/intro.txt`), 14 s; then `Diamond.jpeg` from the
keyboards repo maximized in the image viewer, 3 s.

**Cut.** Keep the intro card under the narration, the photo to the end.

**Narration.**
> This is a video about typing. I use a twenty-four-key keyboard with an
> alternative layout, and I live in vim. Those two never got along — until the editors
> started telling the keyboard which vim mode they're in. The tour, the trick, and the
> install.

### 2 · The problem — 0:37–1:17 (91 words) · `record.py 2`

**Screen (`seg2`).** `alpha1.png` maximized in the image viewer for 20 s, then `vim.png` for
20 s. The narration names the keys (`h|v` the magic key, the `h,` and `mg` combos, `l` on the top
row; then the `h j k l` home row and the left-hand operators); the diagram is legible full-frame.

**Cut.** Switch images where the narration turns to "A vim layer removes the compromise".

**Narration.**
> Every alternative layout breaks vim's motions. On Romak, h is a magic key, j and k are
> two-key combos, l is a top-row reach. Colemak, Dvorak, Gallium — each scatters them
> differently. The usual fixes are bad. Remap vim, and you fight every plugin, tutorial and
> remote machine. Learn the scattered positions, and your best layout is worst at what you do
> most. A vim layer removes the compromise: in normal mode the keyboard isn't typing letters,
> it's issuing commands. The catch was always knowing *when*. Guessing from keystrokes drops
> keys.

### 3 · How it works — 1:17–2:07 (118 words) · `record.py 3`

**Screen (`seg3`).** In the demo terminal, `show channel` (the descriptor bytes and the
eight-code table, `env/cards/channel.txt`) for 12 s, `show pipeline` (the end-to-end diagram)
for 10 s. Then one maximized demo terminal with two tabs: tab 2 runs the log tail, tab 1
types `zmk-vim-mode status`, then `set insert`, `set normal`, `set off`, `set off` again
(back to auto), and a closing `zmk-vim-mode status` showing auto again.

```bash
journalctl --user -u zmk-vim-mode -f -o cat | grep -e decision -e led
```

**Expect.** Banner *Vim normal* → *Vim insert* → *Alpha 1* following each `set`; the closing
`status` shows no override (back to auto); one `led write` line per device in
the tail. `set off` and `set raw` both leave the banner on *Alpha 1*: the closing status
tells them apart.

**Cut.** Keep the two cards under the first six sentences, the `set` sequence under "I can set
it by hand"; drop the `status` dump and the tail's start-up.

**Narration.**
> The editor knows its mode, so the editor should say it. Three parts: an editor plugin, a
> host daemon, and a ZMK module. The channel is one every keyboard already has: the LED
> report the OS uses for Caps Lock and Num Lock. Five bits — and three of them, Compose, Kana
> and Scroll Lock, no operating system ever sets. Read together, that's eight codes, over USB
> or Bluetooth, stock ZMK, no pairing. The daemon writes the code; the firmware flips a layer
> bitmask. I can set it by hand from a terminal and the board follows. And this panel is not a
> mock-up: it reads the keyboard's own reports, so what lights is what the keyboard did.

### 4 · Neovim — 2:07–3:07 (139 words) · `record.py 4`

**Screen.** Ghostty (demo shell) in `showcase/demo-go`. The HUD spells the daemon's modes from
the keyboard's layers: NORMAL → *Vim normal*, INSERT → *Vim insert*, VISUAL → *Vim visual · Vim
normal*, CMDLINE → *Vim cmdline*; **RAW and OFF both read *Alpha 1***.

| Action (what `seg4` types) | HUD banner | daemon reason |
|---|---|---|
| `nvim` → LazyVim dashboard | *Alpha 1* | `raw (code 6) — nvim client` |
| `f` → `modes.go`, Enter | *Vim normal* | `normal (code 1) — nvim client` |
| **the tour**: `j j j k`, `l l l h h`, `w w e b b`, `0 $ 0`, `i` `Esc`, `a` `Esc`, `gg` | *Vim normal* ↔ *Vim insert* | |
| `/Compose` Enter, `A`, type ` // bit 0 of the code`, `Esc`, `u` | *Vim normal* → *Vim insert* → *Vim normal* | `insert` → `normal` |
| `v`, `j`, `j`, `y` | *Vim visual · Vim normal* → *Vim normal* | `visual` → `normal` |
| `:` then `Esc` | *Vim cmdline* → *Vim normal* | `cmdline` → `normal` |
| `<space>` (leader), `Esc` | *Alpha 1* for ~0.3 s | `raw` briefly |
| `<space>ff`, type `readme`, `Esc` `Esc` | *Vim insert* → *Vim normal* | `insert` → `normal` |
| `<space>e` → explorer, `j` `j` `k`, `<space>e` back | *Alpha 1* → *Vim normal* | `raw` → `normal` |
| `:terminal`, `i`, `go test ./...` Enter, `Ctrl-\ Ctrl-n`, `:bd!` | *Alpha 1* → *Vim normal* | `raw` → `normal` |
| `<space>l` → Lazy, `q` | *Alpha 1* → *Vim normal* | `raw` → `normal` |
| `:qa!`, `exit` | *Alpha 1* | `off` |

**Cut.** Keep everything from `nvim` to `q` out of Lazy; drop `:qa!`/`exit`. Trim the tour to
the first eight keys if the words run out.

**Narration.**
> Neovim first, because the plugin reports the real mode from inside the editor. The
> dashboard: every letter here is a command, so the keyboard drops to raw — plain Romak. Open
> a file: normal. i: insert, and Romak is back, untouched. Escape. v, select, yank: visual has
> its own layer. Colon opens the command line on another. Now the part no window-title trick
> can do. Space is my leader — the instant I press it the keyboard goes raw, because the next
> letter is a menu shortcut. The picker is a text field: insert. The file explorer: raw, so a,
> d and r reach the tree instead of vim. The terminal: raw, so Escape belongs to the shell.
> Lazy: raw. Back in the file: normal. The keyboard tracks what the editor is expecting, not
> which window is in front.

### 5 · VS Code — 3:07–3:27 (47 words) · `record.py 5`

**Screen.** `demo-go.code-workspace`, `modes.go`; title bar `modes.go — demo-go [Text Editor]`.

| Action (what `seg5` types) | HUD banner | daemon reason |
|---|---|---|
| `Ctrl+P` `modes.go` Enter, `Esc`, the tour | *Vim normal* ↔ *Vim insert* | `client vscode` |
| `/Kana` Enter, `A`, type ` // bit 1 of the code`, `Esc`; `v`, `Esc` | *Vim insert* / *Vim visual · Vim normal* → *Vim normal* | |
| palette *View: Toggle Terminal*, type `go run ./cmd/vimmode` Enter | *Alpha 1* | `tool window focused: Terminal` |
| palette *View: Toggle Terminal* → back | *Vim normal* | `client vscode` |
| `F1`, type `keyboard`, `Esc` | *Alpha 1* → *Vim normal* | `tool window focused: widget outside any view` |
| `Ctrl+Shift+E` → Explorer, hold the nav layer for `↓` `↓`, `Ctrl+Shift+E` back, `u` | *Alpha 1* → *Vim normal* | `tool window focused: Folders` |

**Cut.** Keep four keys of the tour, the terminal toggle both ways, the palette. Drop the
search-and-comment and the Explorer unless the words allow.

**Narration.**
> The same trick, three more editors. VS Code runs a real Neovim inside it through
> vscode-neovim, so the modes are exact. Everything around the editor comes from the window
> title and a tiny companion extension: the terminal, raw; back in the file, normal; the
> command palette, raw.

### 6 · IntelliJ IDEA — 3:27–3:47 (45 words) · `record.py 6`

**Screen.** `demo-java`, `ModeTable.java`, IdeaVim on. No `Esc` in normal mode (IdeaVim beeps).

| Action (what `seg6` types) | HUD banner | daemon reason |
|---|---|---|
| `Ctrl+Shift+N` `ModeTable` Enter, the tour | *Vim normal* ↔ *Vim insert* | `client intellij` |
| `/COMPOSE` Enter, `Esc`, `A`, type ` // Compose is bit 0`, `Esc`; `v`, `Esc` | *Vim insert* / *Vim visual · Vim normal* → *Vim normal* | |
| `:` then `Esc` | *Alpha 1* → *Vim normal* | `intellij client raw` (IdeaVim's ex line is a separate component) |
| Meh+B → project tool window, type `readme`, `Esc` ×3 | *Alpha 1* → *Vim normal* | `intellij client raw` → `client intellij` |
| `Alt+F12` → terminal, `ls` Enter, `Alt+F12` back, `u` | *Alpha 1* → *Vim normal* | `raw` → `normal` |

**Cut.** Keep four keys of the tour, `i`/`Esc`, the project tree, the terminal.

**Narration.**
> IntelliJ: IdeaVim has a mode listener, so the plugin subscribes to it — insert, visual, the
> same layers. The project tree: raw. The terminal: raw. And without the plugin, the keyboard
> still infers the mode by itself between keystrokes, the way it did for years.

### 7 · Obsidian — 3:47–4:10 (56 words) · `record.py 7`

**Screen.** Obsidian, the *Demo* vault, note *Tasks*.

| Action (what `seg7` types) | HUD banner | daemon reason |
|---|---|---|
| `Esc`, the tour | *Vim normal* ↔ *Vim insert* | `client obsidian` |
| `j j`, `A`, type ` — publish the video`, `Esc`, `u` | *Vim insert* → *Vim normal* | `insert` → `normal` |
| `:` then `Esc` | *Vim cmdline* → *Vim normal* | `cmdline` → `normal` |
| `Ctrl+E` → reading view, `Ctrl+E` back | *Alpha 1* → *Vim normal* | `raw` → `normal` |
| `Ctrl+Shift+F` → search, type `layer`, `Esc`, click back into the note | *Alpha 1* → *Vim normal* | `obsidian client raw` → `normal` |

**Cut.** Keep `A`/type/`Esc`, reading view both ways, search and the click back.

**Narration.**
> Obsidian: CodeMirror's vim, and its own plugin listens to it — normal, insert, the colon
> dialog as the command line. Reading view has no vim, so the board is plain Romak again;
> search, raw; back in the text, normal. Four editors, four different plumbing jobs, one
> daemon — and the keyboard only ever sees a code.

### 8 · Everywhere else — 4:10–4:35 (54 words) · `record.py 8`

**Screen.** The plain demo shell. `seg8` types `zmk-vim-mode set raw` → `status | head -1`
reads `raw (code 6)` with the override; `set raw` again → back to `off`, and a second
`status | head -1` reads `off (code 0)`; then `exit`. The banner reads *Alpha 1*
throughout: this beat is carried by the typed `status` lines.

**Cut.** Keep both `set raw` lines and the `status` lines flipping.

**Narration.**
> Anywhere else, the vim layers are simply off. For what nothing can detect — vim over SSH, a
> screen-share — set is the escape hatch, and repeating it returns to automatic. The keyboard
> listens on USB and Bluetooth at the same time, so the same code reaches it whichever host it
> is paired to.

### 9 · Install and wrap-up — 4:35–5:10 (87 words) · `record.py 9`

**Screen (`seg9`).** In the demo terminal: `doctor` (the demo shell's function — the real
`zmk-vim-mode doctor` with home paths shown as `~`), 8 s; `show node` (the `vim_sync { … }`
devicetree node, `env/cards/node.txt`), 6 s; `show links` (the end card: zmk-vim-mode ·
zmk-layer-hud · keyboards · Romak), 8 s.

**Cut.** Doctor under the first sentence, the node under the second, the links to the end.

**Narration.**
> One command installs the daemon, the service, the Neovim spec and the editor extensions, and
> doctor tells you what's left. The keyboard side is four layers and one devicetree node — they
> can start empty. Everything is on GitHub: the tool, the keymap, the layout, and the layer HUD,
> which works with any ZMK board and doesn't need vim at all. If you type on an alternative
> layout and live in vim, this is the compromise you no longer have to make. Tell me your layout
> below.

---

## Shot list

Every shot is a screen recording of the recording monitor, HUD rail included, produced by
`record.py` from the segment of the same number.

| # | Shot | Source | Beat |
|---|---|---|---|
| A | Cold open in Neovim | `seg0` | 0 |
| B | Title and intro cards, Diamond photo | `env/cards/{title,intro}.txt`; `keyboards/docs/img/builds/Diamond.jpeg` in the image viewer | 1 |
| C | Layer diagrams | `keyboards/docs/img/diagrams/{alpha1,vim}.png` in the image viewer | 2 |
| D | Channel and pipeline cards, then the live `set` sequence | `env/cards/{channel,pipeline}.txt`; one demo terminal with two tabs | 3 |
| E | Editor demos | `seg4`–`seg7` | 4–7 |
| F | Everywhere else | `seg8` | 8 |
| G | Doctor (paths masked), the node card, the links card | the demo shell's `doctor`; `env/cards/{node,links}.txt` | 9 |

## Pre-flight, every run

`bash showcase/prepare.sh` run (it strips the codex/weather bar widgets too) · monitor at
scale 1.25 · take HUD
stopped (`bash showcase/hud.sh stop`; the rehearsal brings its own) · no daemon override
(`zmk-vim-mode status` shows none) · `git status` clean in the demo dirs · parked on
workspace 8 (the driver re-parks before every take; takes never leave workspaces 5–8) ·
the keyboards repo at
`~/projects/keyboards` (or `KEYBOARDS_REPO`) · `imv` present (or `ZMK_IMAGE_VIEWER`). After the
run, scrub each take: HUD drawing the board throughout, no error line in any terminal, no
window from workspaces 1–4, the typed `status` lines matching beats 3 and 8.

## Pass/fail per take

`zmk-vim-mode status` (or the typed `status` lines in beats 3 and 8) must show the reason
in the tables above; the HUD
shows the keyboard's layers, not the daemon's reasoning. `rehearse.py` checks the same reasons
and reports to `run/rehearsal.log`; a take with a failed check is a setup problem (plugin not
loaded, title marker missing, the unit without `--atspi`), not a script problem.

## Words → time

| Beat | Words | Time |
|---|---|---|
| 0 | 57 | 0:18 |
| 1 | 45 | 0:19 |
| 2 | 91 | 0:40 |
| 3 | 118 | 0:50 |
| 4 | 139 | 1:00 |
| 5 | 47 | 0:20 |
| 6 | 45 | 0:20 |
| 7 | 56 | 0:23 |
| 8 | 54 | 0:25 |
| 9 | 87 | 0:35 |
| **total** | **739** | **5:10** |

The automated segments run longer than their words by design (they exercise every state the
rehearsal checks); the *Cut* lines say what to keep. Cut the actions, not the words.
