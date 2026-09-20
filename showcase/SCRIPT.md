# The keyboard that knows your vim mode — video script

**Format:** screen recording plus English narration, nothing else — no camera, no animation, no
music. 10:08 as assembled. **Every beat is recorded hands-off** by `record.py`; the narration is
written against the cut picture and dubbed by `dub.py`, which places each cued line at its
own moment inside the take.
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

### 0 · Cold open — 0:00–0:47 (80 words) · `record.py 0`

**Screen (`seg0`).** `nvim internal/modes/modes.go` in the demo shell, `:21` (the comment on the
three indicator bits), `A` → INSERT, the HUD reads *Vim insert*; type ` no OS ever sets these`,
pause, `Esc`. Then `j j k`, `l l h`, at the take pace; `u`, `:qa!`, `exit`. No title first: the
flip is the opening shot.

**Cut.** From the first typed letter to the last `h`; drop the file opening and the exit.

**Expect.** HUD *Vim insert* → *Vim normal* on `Esc`; on the vim layer the right home row
lights index → pinky.

**Narration.**
> [+0.0] Watch the panel on the right. That's my keyboard, twenty-four keys, a layout called
> Romak, and it's drawn live from the board's own reports, so what lights up is what the
> keyboard actually did.
> [+15.3] The file is open, and vim is in normal mode.
> [+20.8] A, and we're inserting. Romak is back, laid out for letters.
> [+26.5] Escape. The layers change, and h, j, k and l are under my right hand.
> [+33.7] Nothing in vim was remapped. The keyboard followed the editor.

### 1 · Title and intro — 0:47–1:37 (84 words) · `record.py 1`

**Screen (`seg1`).** `show title` in the demo terminal (the card in `env/cards/title.txt`),
4 s; `show intro` (the card in `env/cards/intro.txt`), 14 s; then `Diamond.jpeg` from the
keyboards repo maximized in the image viewer, 3 s.

**Cut.** Keep the intro card under the narration, the photo to the end.

**Narration.**
> [+0.0] This is a video about typing.
> [+4.5] I use a twenty-four-key keyboard with an alternative layout, and I live in vim. Those
> two never got along, for a simple reason: vim's motions are placed for QWERTY, and an
> alternative layout moves them somewhere else. What follows isn't a compromise between them,
> and it isn't a remap either. The editors tell the keyboard which vim mode they're in, and the
> keyboard changes layers to match.
> [+33.0] This is the board. The tour, the trick, and the install.

### 2 · The problem — 1:37–2:34 (104 words) · `record.py 2`

**Screen (`seg2`).** `alpha1.png` maximized in the image viewer for 20 s, then `vim.png` for
20 s. The narration names the keys (`h|v` the magic key, the `h,` and `mg` combos, `l` on the top
row; then the `h j k l` home row and the left-hand operators); the diagram is legible full-frame.

**Cut.** Switch images where the narration turns to "A vim layer removes the compromise".

**Narration.**
> [+0.0] Every alternative layout breaks vim's motions.
> [+5.4] This is Romak's base layer. On it, h is a magic key, j and k are two-key combos, and
> l is a reach to the top row. Colemak, Dvorak, Gallium: every one of them scatters those four
> keys somehow, and none of them put them back.
> [+26.5] A vim layer removes the compromise. In normal mode the keyboard isn't typing letters
> at all, it's issuing commands, and they sit where they belong. The usual fixes are worse:
> remap vim and you fight every plugin and every remote machine.
> [+46.8] The catch was always knowing when. Guessing from keystrokes drops keys.

### 3 · How it works — 2:34–3:53 (148 words) · `record.py 3`

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
> [+0.0] The editor knows its mode. So the editor should say it. Three parts: a plugin in the
> editor, a daemon on the host, and a module in the keyboard's firmware. The channel between
> them is one every keyboard already has, the LED report the operating system uses for Caps
> Lock and Num Lock. Five bits, and three of them, Compose, Kana and Scroll Lock, no operating
> system ever sets on its own. That's eight codes, over USB or Bluetooth, on stock ZMK, with
> nothing to pair and nothing the operating system has to cooperate with.
> [+39.2] Set it by hand, and the board follows. Insert.
> [+45.4] Normal, and the command layer is back.
> [+51.7] Off, and the vim layers step aside entirely. The daemon writes the code, the firmware
> flips a layer bitmask, and that's the whole protocol.
> [+62.5] And the panel is not a mock-up. It reads the keyboard's own reports.

### 4 · Neovim — 3:53–5:20 (140 words) · `record.py 4`

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
> [+0.0] Neovim first, because here the plugin reports the real mode from inside the editor.
> The dashboard: every letter on it is a command, so the keyboard drops to raw.
> [+16.2] Open a file, press i, and Romak is back.
> [+20.6] Escape, and the command layer returns. The motions are where they should be now: j
> and k, w and b, zero and dollar, single keys under the fingers, not combos.
> [+38.2] Append, type, escape, undo.
> [+40.4] Select and yank: visual has a layer of its own, and the colon line another.
> [+48.9] Insert again, for the comment.
> [+53.5] Space is my leader, so the board goes raw,
> [+56.6] because the next letter is a shortcut, not a motion.
> [+61.7] The explorer is raw too, so a, d and r reach the tree.
> [+67.9] Back in the file, normal again.
> [+74.4] The terminal is raw, so Escape belongs to the shell.

### 5 · VS Code — 5:20–6:37 (140 words) · `record.py 5`

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
> [+0.0] The same trick, three more editors.
> [+3.0] VS Code is the interesting one, because inside it there are four separate ways to tell
> what's going on.
> [+11.5] The mode itself is exact, because vscode-neovim runs a real Neovim inside VS Code,
> which loads my config, which loads the same plugin as before. None of it is VS Code specific.
> [+29.0] Insert, visual, normal, all of them exact.
> [+36.7] But no extension API fires when focus moves to the terminal or the sidebar. So the
> window title carries it: a marker reading Text Editor in the editor, and the view's own name
> anywhere else. Terminal, raw.
> [+56.3] Back in the file, normal.
> [+58.9] The palette, raw again.
> [+60.8] A companion extension covers those quick inputs, where a letter is a shortcut and not
> a motion.
> [+67.5] And on Linux the accessibility bus catches whatever you open with the mouse.

### 6 · IntelliJ IDEA — 6:37–7:37 (107 words) · `record.py 6`

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
> [+0.0] IntelliJ. IdeaVim has a mode listener, so the plugin subscribes to it directly.
> [+6.5] Same layers as everywhere else. One thing it deliberately doesn't report is operator
> pending: when c is waiting for a motion, the keyboard's own gesture layer owns that moment,
> and the daemon stays out of it.
> [+22.0] And without the plugin nothing breaks: the keyboard infers the mode between
> keystrokes, as it did for years.
> [+31.0] i for insert, and Escape back to normal.
> [+39.3] The colon line is a separate component, so, raw.
> [+45.5] The project tree, raw.
> [+49.0] And the terminal, though that one takes work: in IntelliJ the consoles are editors
> too, told apart by kind.

### 7 · Obsidian — 7:37–8:26 (88 words) · `record.py 7`

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
> [+0.0] Obsidian runs CodeMirror's vim, and its own plugin listens to that.
> [+7.3] Normal mode, and the same command layer as the other three. The colon dialog becomes
> the command line, exactly the way it does in Neovim, because as far as the daemon is
> concerned it is the same code travelling the same wire.
> [+25.3] Insert, and Romak returns.
> [+33.6] Reading view has no vim at all, so the board goes plain. Search, raw.
> [+38.5] Four editors, four different plumbing jobs, one daemon, and a keyboard that only ever
> sees a number.

### 8 · Everywhere else — 8:26–9:18 (115 words) · `record.py 8`

**Screen.** The plain demo shell. `seg8` types `zmk-vim-mode set raw` → `status | head -1`
reads `raw (code 6)` with the override; `set raw` again → back to `off`, and a second
`status | head -1` reads `off (code 0)`; then `exit`. The banner reads *Alpha 1*
throughout: this beat is carried by the typed `status` lines.

**Cut.** Keep both `set raw` lines and the `status` lines flipping.

**Narration.**
> [+0.0] Anywhere else, the vim layers are simply off. The keyboard is just a keyboard.
> [+6.8] And for what nothing can detect, vim over ssh, a shared screen, a machine that isn't
> yours, set is the escape hatch. It pins the mode by hand, and the keyboard believes it until
> you say otherwise. Run it again and the override clears, and the daemon goes back to deciding
> for itself. There is no mode to remember and nothing to undo later. The board also listens on
> USB and Bluetooth at the same time, so the same code reaches it whichever host it's paired
> to, and switching hosts changes nothing.
> [+43.0] No pairing step, and no per-host setup.

### 9 · Install and wrap-up — 9:18–10:08 (93 words) · `record.py 9`

**Screen (`seg9`).** In the demo terminal: `doctor` (the demo shell's function — the real
`zmk-vim-mode doctor` with home paths shown as `~`), 8 s; `show node` (the `vim_sync { … }`
devicetree node, `env/cards/node.txt`), 6 s; `show links` (the end card: zmk-vim-mode ·
zmk-layer-hud · keyboards · Romak), 8 s.

**Cut.** Doctor under the first sentence, the node under the second, the links to the end.

**Narration.**
> [+0.0] One command installs the daemon, the service, the Neovim spec and the editor
> extensions.
> [+6.9] And doctor tells you what's left to do by hand, one line per thing, with the fix next
> to it.
> [+22.4] The keyboard side is four layers and one devicetree node, and they can start
> completely empty. Nothing breaks while they are.
> [+32.0] Everything is on GitHub: the tool, the keymap, the layout, and the layer HUD you have
> been watching.
> [+39.5] If you type on an alternative layout and live in vim, this is for you. Tell me your
> layout below.

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

| Beat | Words | Starts | Runs |
|---|---|---|---|
| 0 | 80 | 0:00 | 48s |
| 1 | 84 | 0:47 | 50s |
| 2 | 104 | 1:37 | 58s |
| 3 | 148 | 2:34 | 79s |
| 4 | 140 | 3:53 | 87s |
| 5 | 140 | 5:20 | 76s |
| 6 | 107 | 6:37 | 60s |
| 7 | 88 | 7:37 | 49s |
| 8 | 115 | 8:26 | 52s |
| 9 | 93 | 9:18 | 50s |
| **total** | **1099** | | **10:08** |

The automated segments run longer than their words by design (they exercise every state the
rehearsal checks); the *Cut* lines say what to keep. Cut the actions, not the words.
