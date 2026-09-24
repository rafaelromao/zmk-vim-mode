# The keyboard that knows your vim mode — video script

**Format:** screen recording plus English narration, nothing else — no camera, no animation, no
music. 8:08 once `dub.py tighten` has cut the dead air and played the holds (10:50 before it). **Every beat is recorded hands-off** by `record.py`; the narration is
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
the complete existing menu-bar layout and icons left unchanged, the take HUD stopped
(`bash showcase/hud.sh stop`), and `bash showcase/prepare.sh` done. The driver parks on
workspace 8 before every take and the segments never leave workspaces 5–8, so nothing
below workspace 5 is ever on camera. `ZMK_RECORD_FPS=60` for the deliverable.

**Hard rule for every take: no combos for literal characters.** Every printable character entered
into a shell, editor, document, or Vim command line must resolve as a single key on the active
layer or through its proper layer—not a COMBO pill. This applies to letters, digits, punctuation,
and symbols. If a character has no single-key/layer route in the modeled keymap, change the demo
string or fix the layer model before recording; never fall back to its combo.

Typing pace is 60 words per minute — about one character every 0.20 s. Sticky Alpha 2 characters
add a 0.10 s thumb-hop pause; Vim command motions use the base gap. The `q k y z x w j` letters,
accents, `ç`, `'` and `_` use the sticky `alpha2` thumb; `h|v` produces `h` at a word start or
after a consonant and `v` after a vowel, while `v|h` on `alpha2` supplies the opposite result.
Uppercase alpha2 letters use `shifted2`; uppercase alpha1 letters tap sticky shift. Literal digits
and numeric-layer punctuation use NUMBERS; symbols use SYMBOLS (or NUMBERS where the keymap puts
them). Plain Alpha 1 punctuation remains a single key. The feed restores the exact prior layer
stack after each held-layer character. During capture, `record.py` temporarily sets the HUD
`press_ms` to 100 so Alpha 2 returns to Alpha 1 before the next character, then restores the user's
saved setting. Non-text control actions may use their documented keymap combos, including Enter,
Tab, Escape, and the modifier shortcuts named in the segment. Those are commands, not character
entry; no combo may be used to emit a literal letter, digit, punctuation mark, or symbol.

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

### 0 · Cold open — 0:00–0:38 (93 words) · `record.py 0`

**Screen (`seg0`).** `nvim internal/modes/modes.go` in the demo shell, `:21` (the comment on the
three indicator bits), `A` → INSERT, the HUD reads *Vim insert*; type ` no OS ever sets these`,
pause, `Esc`. Then `j j k`, `l l h`, at the take pace; `u`, `:qa!`, `exit`. No title first: the
flip is the opening shot.

**Cut.** From the first typed letter to the last `h`; drop the file opening and the exit.

**Expect.** HUD *Vim insert* → *Vim normal* on `Esc`; on the vim layer the right home row
lights index → pinky.

**Timeline** (cue seconds, 2026-09-23 take): terminal +4.8 · file open, normal +15.9 · `:21`
+19.3 · `A`, insert +21.1 · Esc, normal +27.9 · `h j k l` +28.8–+32.3 · `u` +33.2 · `:qa!` +33.8
· HUD gone +39.2.

**Hold** +16.3 for 5.7 s — the file has just opened and nothing moves until `:21`, so the
introduction can finish and "The file is open" is said over the open file, before the `A`.

**Narration.**
> [+5.2] Hi, in this video I'm going to show you how I use my keyboards with vim editors.
> Watch the panel on the right. That's my keyboard: twenty-four keys, a layout called Romak,
> drawn live from its own reports. What lights up is what the keyboard actually did.
> [+17.3] The file is open, and vim is in normal mode.
> [+21.1] A, and we're inserting. Romak is back, laid out for letters.
> [+27.9] Escape. The layers change, and h, j, k and l are under my right hand.
> [+33.2] Nothing in vim was remapped. The keyboard followed the editor.

### 1 · Title and intro — 0:38–1:12 (84 words) · `record.py 1`

**Screen (`seg1`).** `show title` in the demo terminal (the card in `env/cards/title.txt`),
4 s; `show intro` (the card in `env/cards/intro.txt`), 14 s; then `Diamond.jpeg` from the
keyboards repo maximized in the image viewer, 3 s.

**Cut.** Keep the intro card under the narration, the photo to the end.

**Timeline** (cue seconds): terminal +4.5 · title card ~+12 · intro card ~+18 · Diamond photo
+33.5 to +36.5 · HUD gone +40.2.

**Narration.**
> [+5.4] This is a video about typing. I use a twenty-four-key keyboard with an alternative
> layout, and I live in vim. Those two never got along, for a simple reason: vim's motions are
> placed for QWERTY, and an alternative layout moves them somewhere else. What follows isn't a
> compromise between them, and it isn't a remap either. The editors tell the keyboard which vim
> mode they're in, and the keyboard changes layers to match.
> [+33.4] This is the board. The tour, the trick, and the install.

### 2 · The problem — 1:12–2:01 (125 words) · `record.py 2`

**Screen (`seg2`).** `alphas.png`, with Alpha 1 and Alpha 2 side by side, maximized in the image
viewer for 20 s, then `vim.png` for 20 s. The narration names the keys (`h|v` the magic key, the
`h,` and `mg` combos, `l` on the top row; then the `h j k l` home row and the left-hand operators);
the diagram is legible full-frame.

**Cut.** Switch images where the narration turns to "A vim layer removes the compromise".

**Timeline** (cue seconds): `alphas.png` +4.9 to +25.3 · `vim.png` +26.0 to +46.3 · HUD gone
+47.0.

**Hold** +24.5 for 6.3 s — `alphas.png` is a still image; the base-layer line is 25.6 s and the
image is on screen for 20.

**Hold** +45.5 for 1.3 s — `vim.png`, still, so the last line ends before the image closes.

**Narration.**
> [+5.6] Every alternative layout breaks vim's motions. This is Romak's base layer. On it, h is
> a magic key, l is a reach to the top row, and j and k aren't on this layer at all: each one is
> either a combo, or a hop to the second alpha layer. That's harder than some other alternative
> layouts make it. Colemak, Dvorak and Gallium all scatter those four keys; Romak just scatters
> them further.
> [+26.6] A vim layer removes the compromise. In normal mode the keyboard isn't typing letters
> at all, it's issuing commands, and they sit where they belong. The usual fixes are worse:
> remap vim and you fight every plugin and every remote machine.
> [+43.0] The catch was always knowing when. Guessing from keystrokes drops keys.

### 3 · How it works — 2:01–3:02 (148 words) · `record.py 3`

**Screen (`seg3`).** In the demo terminal, `show channel` (the descriptor bytes and the
eight-code table, `env/cards/channel.txt`) for 12 s, `show pipeline` (the end-to-end diagram)
for 10 s. Then one maximized demo terminal split down: the bottom pane runs the log tail while
the top pane types `zmk-vim-mode status`, then `set insert`, `set normal`, `set off`, `set off` again
(back to auto), and a closing `zmk-vim-mode status` showing auto again.

```bash
journalctl --user -u zmk-vim-mode -f -o cat | grep --line-buffered -e decision -e led | cut -c 51-
```

**Expect.** Banner *Vim normal* → *Vim insert* → *Alpha 1* following each `set`; the closing
`status` shows no override (back to auto); one short `decision` and one `led write` line per
device appears in the lower pane beside each command. `set off` and `set raw` both leave the
banner on *Alpha 1*: the closing status tells them apart.

**Cut.** Keep the two cards under the first six sentences, the `set` sequence under "I can set
it by hand"; drop the `status` dump and the tail's start-up.

**Known gap in the 2026-09-23 take:** no cards. `seg3` has never printed `show channel` or
`show pipeline` — the 2026-09-19 edit that was meant to add them did not apply, and nothing
checked. The channel paragraph therefore runs over the split and the log tail being typed.
Adding the two cards to `seg3` needs a new take 3 and new beat-3 cues.

**Timeline** (cue seconds): terminal +4.4 · split and log tail typed +9.9–+32.5 · `status` dump
+40.4 · `set insert`, insert +49.65 · `set normal`, normal +57.1 · `set off`, Alpha 1 +63.9 ·
closing `status` +76.4 · HUD gone +82.5.

**Narration.**
> [+5.3] The editor knows its mode. So the editor should say it. Three parts: a plugin in the
> editor, a daemon on the host, and a module in the keyboard's firmware. The channel between
> them is one every keyboard already has, the LED report the operating system uses for Caps
> Lock and Num Lock. Five bits, and three of them, Compose, Kana and Scroll Lock, no operating
> system ever sets on its own. That's eight codes, over USB or Bluetooth, on stock ZMK, with
> nothing to pair and nothing the operating system has to cooperate with.
> [+47.4] Set it by hand, and the board follows. Insert.
> [+57.1] Normal, and the command layer is back.
> [+63.9] Off, and the vim layers step aside entirely. The daemon writes the code, the firmware
> flips a layer bitmask, and that's the whole protocol.
> [+73.2] And the panel is not a mock-up. It reads the keyboard's own reports.

### 4 · Neovim — 3:02–4:18 (148 words) · `record.py 4`

**Screen.** Ghostty (demo shell) in `showcase/demo-go`. The HUD spells the daemon's modes from
the keyboard's layers: NORMAL → *Vim normal*, INSERT → *Vim insert*, VISUAL → *Vim visual · Vim
normal*, CMDLINE → *Vim cmdline*; **RAW and OFF both read *Alpha 1***.

| Action (what `seg4` types) | HUD banner | daemon reason |
|---|---|---|
| `nvim` → LazyVim dashboard | *Alpha 1* | `raw (code 6) — nvim client` |
| `f` → `modes.go`, Enter | *Vim normal* | `normal (code 1) — nvim client` |
| **the tour**: `j j j k`, `l l l h h`, `w w e b b`, `0 $ 0`, `i` `Esc`, `a` `Esc`, `gg` | *Vim normal* ↔ *Vim insert* | |
| `/Compose` Enter, `A`, type ` // bit 0 of the code`, `Esc`, `u` | *Vim normal* → *Vim insert* → *Vim normal* | `insert` → `normal` |
| `Esc` reset, Alpha 2 `v|h` → `v`, `j`, `j`, Alpha 2 `y` | *Vim visual · Vim normal* → *Vim normal* | `visual` → `normal` |
| `:` then `Esc` | *Vim cmdline* → *Vim normal* | `cmdline` → `normal` |
| `<space>` (leader), `Esc` | *Alpha 1* for ~0.3 s | `raw` briefly |
| `<space>ff`, type `readme`, `Esc` `Esc` | *Vim insert* → *Vim normal* | `insert` → `normal` |
| `<space>e` → explorer, `j` `j` `k`, `<space>e` back | *Alpha 1* → *Vim normal* | `raw` → `normal` |
| `:terminal`, `i`, `go test ./...` Enter, `Ctrl-\ Ctrl-n`, `:bd!` | *Alpha 1* → *Vim normal* | `raw` → `normal` |
| `<space>l` → Lazy, `q` | *Alpha 1* → *Vim normal* | `raw` → `normal` |
| `:qa!`, `exit` | *Alpha 1* | `off` |

**Cut.** Keep everything from `nvim` to `q` out of Lazy; drop `:qa!`/`exit`. Trim the tour to
the first eight keys if the words run out.

**Timeline** (cue seconds): dashboard +10.0 · picker, insert +14.5 · file open, normal +19.55 ·
tour +20.0–+30.5 · `i` +30.9 · `A`, insert +37.5 · Esc +43.5 · `u` +44.0 · `v` +45.0 · `j j`
+45.8 · `y` +46.5 · `:` +47.4 · leader +48.8 and +49.5 · picker, insert +50.05 · explorer
+54.7–+58.8 · terminal, raw +63.75–+70.6 · Lazy +72.8–+74.3 · HUD gone +81.8.

**Hold** +13.0 for 2.2 s — the dashboard is still, so "drops to raw" ends before the picker opens.

**Hold** +48.4 for 1.2 s — the colon line has closed and nothing moves until the leader, so
"Space is my leader" can start on the first Space instead of after the picker opens.

**Hold** +57.8 for 1.4 s — the explorer rests after `j j k`, so its line ends before it closes.

**Narration.**
> [+5.4] Neovim first, because here the plugin reports the real mode from inside the editor.
> The dashboard: every letter on it is a command, so the keyboard drops to raw.
> [+19.2] Open a file, and it's normal.
> [+21.4] The motions are where they should be now: j and k, w and b, zero and dollar, single
> keys under the fingers, not combos.
> [+30.9] i for insert, and Romak is back, untouched. Escape.
> [+37.5] Append, and the comment goes in as letters.
> [+43.5] Escape, undo.
> [+45.0] v, select, yank: visual gets its own layer, and the colon line another.
> [+48.9] Space is my leader, so the board goes raw: the next letter is a shortcut.
> [+55.1] The explorer is raw too, so a, d and r reach the tree instead of vim.
> [+63.5] The terminal is raw, so Escape belongs to the shell.
> [+73.4] Lazy, raw again. The keyboard tracks what the editor expects, not which window is in
> front.

### 5 · VS Code — 4:18–5:22 (140 words) · `record.py 5`

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

**Timeline** (cue seconds): quick open +6.2 · file, normal +9.4 · tour +10–+21.5 · `A`, insert
+28.25 · Esc +34.1 · `v` +34.6 · Esc +35.4 · palette +36.4 · terminal open +42.5 (its `go run`
lost the `g`: `bash: command not found: o` is on screen until +58) · palette +52.2 · file,
normal +59.15 · palette +61.15 · Esc +65.25 · Explorer +66.15–+68.35 · HUD gone +70.35.

**Hold** +64.5 for 3.9 s — the palette rests with `keyboard` typed, so the companion-extension
line is said over it and the last line still ends before the HUD goes.

**Narration.**
> [+5.0] The same trick, three more editors. VS Code is the interesting one, because inside it
> there are four separate ways to tell what's going on.
> [+14.6] The mode itself is exact, because vscode-neovim runs a real Neovim inside VS Code,
> which loads my config, which loads the same plugin as before. None of it is VS Code specific.
> [+34.0] Insert, visual, normal, all of them exact.
> [+37.1] But no extension API fires when focus moves to the terminal or the sidebar. So the
> window title carries it: a marker reading Text Editor in the editor, and the view's own name
> anywhere else. Terminal, raw.
> [+59.1] Back in the file, normal.
> [+61.1] The palette, raw again.
> [+62.9] A companion extension covers those quick inputs, where a letter is a shortcut and not
> a motion.
> [+65.5] And on Linux the accessibility bus catches whatever you open with the mouse.

### 6 · IntelliJ IDEA — 5:22–6:06 (101 words) · `record.py 6`

**Screen.** `demo-java`, `ModeTable.java`, IdeaVim on. No `Esc` in normal mode (IdeaVim beeps).

| Action (what `seg6` types) | HUD banner | daemon reason |
|---|---|---|
| `Ctrl+Shift+N` `ModeTable` Enter, the tour | *Vim normal* ↔ *Vim insert* | `client intellij` |
| `/COMPOSE` Enter, `Esc`, `A`, type ` // Compose is bit 0`, `Esc`; `v`, `Esc` | *Vim insert* / *Vim visual · Vim normal* → *Vim normal* | |
| `:` then `Esc` | *Alpha 1* → *Vim normal* | `intellij client raw` (IdeaVim's ex line is a separate component) |
| Meh+B → project tool window, type `readme`, `Esc` ×3 | *Alpha 1* → *Vim normal* | `intellij client raw` → `client intellij` |
| `Alt+F12` → terminal, `ls` Enter, `Alt+F12` back, `u` | *Alpha 1* → *Vim normal* | `raw` → `normal` |

**Cut.** Keep four keys of the tour, `i`/`Esc`, the project tree, the terminal.

**Timeline** (cue seconds): quick open +7.0 · file, normal +10.9 · tour +11.9–+22 · `/COMPOSE`
(search bar raw) +26.3 · `A`, insert +30.25 · Esc +35.8 · `v`, visual +36.4 · `:`, raw +37.8 ·
Esc +38.6 · project tree +39.4–+44.1 · terminal +45.75–+49.65 · `u` ~+50.4 · HUD gone +51.6.

**Hold** +48.6 for 3.5 s — the terminal rests after `ls`, so the consoles line ends while it is
still open.

**Hold** +51.0 for 2.3 s — the editor is still after `u`, so the last line ends before the HUD
goes.

**Narration.**
> [+5.0] IntelliJ. IdeaVim has a mode listener, so the plugin subscribes to it directly.
> [+11.2] Same layers as everywhere else, and the same keys under the fingers.
> [+15.8] One thing it deliberately doesn't report is operator pending: when c waits for a
> motion, the keyboard's own gesture layer owns it.
> [+30.2] Insert, and the letters come back.
> [+36.4] Visual, then the colon line, which is a separate component, so, raw.
> [+41.0] The project tree, raw as well.
> [+45.8] And the terminal, though that one takes work: in IntelliJ the consoles are editors
> too, told apart by kind.
> [+49.9] And without the plugin, the keyboard still infers the mode itself.

### 7 · Obsidian — 6:06–6:45 (88 words) · `record.py 7`

**Screen.** Obsidian, the *Demo* vault, note *Tasks*.

| Action (what `seg7` types) | HUD banner | daemon reason |
|---|---|---|
| `Esc`, the tour | *Vim normal* ↔ *Vim insert* | `client obsidian` |
| `j j`, `A`, type ` — publish the video`, `Esc`, `u` | *Vim insert* → *Vim normal* | `insert` → `normal` |
| `:` then `Esc` | *Vim cmdline* → *Vim normal* | `cmdline` → `normal` |
| `Ctrl+E` → reading view, `Ctrl+E` back | *Alpha 1* → *Vim normal* | `raw` → `normal` |
| `Ctrl+Shift+F` → search, type `layer`, `Esc`, click back into the note | *Alpha 1* → *Vim normal* | `obsidian client raw` → `normal` |

**Cut.** Keep `A`/type/`Esc`, reading view both ways, search and the click back.

**Timeline** (cue seconds): note, normal +5.2 · tour +8.6–+18 · `A`, insert +23.8 · Esc +29.2 ·
`u` +29.8 · `:`, cmdline +30.4 · Esc +31.2 · reading view +31.8–+33.2 · search +34.4 · click back,
normal +37.85 · HUD gone +40.1.

**Hold** +32.6 for 2.3 s — reading view is up for only 1.2 s; its frame is still, so "so the
board goes plain" is said over it instead of over the banner turning normal again.

**Hold** +38.5 for 3.2 s — the note is still after the click, so the closing line ends before the
HUD goes.

**Narration.**
> [+5.4] Obsidian runs CodeMirror's vim, and its own plugin listens to that. Normal mode, and
> the same command layer as the other three. The colon dialog becomes the command line, exactly
> the way it does in Neovim, because as far as the daemon is concerned it is the same code
> travelling the same wire.
> [+26.4] Insert, and Romak returns.
> [+31.8] Reading view has no vim at all, so the board goes plain.
> [+34.6] Search, raw.
> [+36.6] Four editors, four different plumbing jobs, one daemon, and a keyboard that only ever
> sees a number.

### 8 · Everywhere else — 6:45–7:30 (115 words) · `record.py 8`

**Screen.** The plain demo shell. `seg8` types `zmk-vim-mode set raw` → `status | head -1`
reads `raw (code 6)` with the override; `set raw` again → back to `off`, and a second
`status | head -1` reads `off (code 0)`; then `exit`. The banner reads *Alpha 1*
throughout: this beat is carried by the typed `status` lines.

**Cut.** Keep both `set raw` lines and the `status` lines flipping.

**Timeline** (cue seconds): terminal +4.7 · `set raw` Enter +14.25 · `status` Enter +23.0 ·
second `set raw` Enter +31.5 (override cleared) · `status` Enter +40.25 · `exit` ~+43 · HUD gone
+45.85.

**Hold** +41.0 for 10.3 s — the final `status` line (`off (code 0)`) rests on screen while the
USB-and-Bluetooth line and the closing sentence are said; the terminal does not change.

**Narration.**
> [+5.3] Anywhere else, the vim layers are simply off. The keyboard is just a keyboard.
> [+10.4] And for what nothing can detect, vim over ssh, a shared screen, a machine that isn't
> yours, set is the escape hatch. It pins the mode by hand, and the keyboard believes it until
> you say otherwise.
> [+29.8] Run it again and the override clears, and the daemon goes back to deciding for itself.
> There is no mode to remember and nothing to undo later.
> [+39.5] The board also listens on USB and Bluetooth at the same time, so the same code reaches
> it whichever host it's paired to, and switching hosts changes nothing. No pairing step, and no
> per-host setup.

### 9 · Install and wrap-up — 7:30–8:08 (93 words) · `record.py 9`

**Screen (`seg9`).** In the demo terminal: `doctor` (the demo shell's function — the real
`zmk-vim-mode doctor` with home paths shown as `~`), 8 s; `show node` (the `vim_sync { … }`
devicetree node, `env/cards/node.txt`), 6 s; `show links` (the end card: zmk-vim-mode ·
zmk-layer-hud · keyboards · Romak), 8 s.

**Cut.** Doctor under the first sentence, the node under the second, the links to the end.

**Timeline** (cue seconds): terminal +4.8 · `doctor` output +10.8 · node card +21.3 · links card
+30.1 · `exit` ~+37.8 · HUD gone +40.95.

**Hold** +37.5 for 6.2 s — the links card is the end card; it stays up while the closing line is
said.

**Narration.**
> [+5.3] One command installs the daemon, the service, the Neovim spec and the editor
> extensions. And doctor tells you what's left to do by hand, one line per thing, with the fix
> next to it.
> [+21.5] The keyboard side is four layers and one devicetree node, and they can start
> completely empty. Nothing breaks while they are.
> [+30.4] Everything is on GitHub: the tool, the keymap, the layout, and the layer HUD you have
> been watching.
> [+37.4] If you type on an alternative layout and live in vim, this is for you. Tell me your
> layout below.

---

## Shot list

Every shot is a screen recording of the recording monitor, HUD rail included, produced by
`record.py` from the segment of the same number.

| # | Shot | Source | Beat |
|---|---|---|---|
| A | Cold open in Neovim | `seg0` | 0 |
| B | Title and intro cards, Diamond photo | `env/cards/{title,intro}.txt`; `keyboards/docs/img/builds/Diamond.jpeg` in the image viewer | 1 |
| C | Layer diagrams | `keyboards/docs/img/diagrams/{alphas,vim}.png` in the image viewer | 2 |
| D | Channel and pipeline cards, then the live `set` sequence | `env/cards/{channel,pipeline}.txt`; one demo terminal split into two panes | 3 |
| E | Editor demos | `seg4`–`seg7` | 4–7 |
| F | Everywhere else | `seg8` | 8 |
| G | Doctor (paths masked), the node card, the links card | the demo shell's `doctor`; `env/cards/{node,links}.txt` | 9 |

## Pre-flight, every run

`bash showcase/prepare.sh` run (it preserves every existing menu-bar icon and widget) · monitor at
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
| 0 | 93 | 0:00 | 38s |
| 1 | 84 | 0:38 | 33s |
| 2 | 125 | 1:12 | 50s |
| 3 | 148 | 2:01 | 61s |
| 4 | 148 | 3:02 | 76s |
| 5 | 140 | 4:18 | 65s |
| 6 | 101 | 5:22 | 44s |
| 7 | 88 | 6:06 | 39s |
| 8 | 115 | 6:45 | 45s |
| 9 | 93 | 7:30 | 37s |
| **total** | **1135** | | **8:08** |

The automated segments run longer than their words by design (they exercise every state the
rehearsal checks); the *Cut* lines say what to keep. Cut the actions, not the words.
