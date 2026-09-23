# Handoff — take 3: make the HUD type like the Diamond's owner, then re-record

Written 2026-09-22 after the user reviewed `showcase/run/showcase.mp4` (7:46, 2560×1440 @ 60 fps,
dubbed). The pipeline is done and green: `record.py all` records ten beats hands-off, `dub.py`
cuts the dead air and lays the narration in sync. **The video is rejected on content, not on
process**, for three reasons the user named and a handful this review adds. Fix them, re-run the
same pipeline, and check the result frame by frame before calling it done.

**Update 2026-09-23:** a later rerun passed the HUD assertions, but its opening retained a
sliver of the previous attempt. The recording driver focused workspace 8 before `seg0` opened
its fresh Ghostty; the prior demo terminal was still there during recorder pre-roll. The current
rerun also adds this spoken opening, verbatim: “Hi, in this video I'm going to show you how I use
my keyboards with vim editors”. Clean workspace 8 before every take and frame-check the first
seconds of the tightened master before accepting it.

Read first: `SCRIPT.md` (what each segment types), `TAKE-2-PLAN.md` (why the video is shaped
this way), `HANDOFF-TAKE-2.md` (how the box is set up: scale 1.25, bar widgets, Ghostty tabs,
mode line, TTS), `HANDOFF.md` (everything older). Do not re-derive any of it.

---

## 1 · What is wrong with `showcase.mp4`

The user's words, then the evidence.

### 1.1 Letters from the secondary alpha layer are drawn as combos

> "it is important that letters from the secondary alpha layer are typed using the secondary
> alpha layer, not combos"

Romak on the Diamond has two alpha layers. `alpha1` carries `b m g l o u / d n s t r a e i /
f c p h|v , .`; everything else — **q, k, y, z, x, w, j, v** (as the reversed magic key), the
accented vowels, `ç`, `'` and `_` — lives on **`alpha2`**, reached by tapping the sticky
`alpha2` thumb key (third thumb, `{ t: alpha2, s: sticky, h: symbols }` in the drawer YAML): the
layer is on for one key and drops back. The keymap *also* lists two-key combos on `alpha1` that
produce those letters (`k x q w z y v j`, and their capitals on `shifted1`), and that is what the
HUD drew every time the takes typed one of them: a **COMBO pill** with two keys lit, no alpha2
thumb, no *Alpha 2* banner.

Seen in the video: the `k`/`K` combo pill near `b m` during `/Kana` in VS Code (~4:17), the same
pill during `jj`/`publish the video` in Obsidian (~6:0x), and every `w`, `y`, `k` in the demo
comments. The user types none of these as combos.

**Required on camera**, for each alpha2 letter: the `alpha2` thumb flashes, the banner reads
*Alpha 2* for that one key, the letter's key on the alpha2 drawer flashes, the banner returns to
the layer the keyboard is really on. Enter and Tab as combos are fine (they are combos on the
Diamond).

### 1.2 The magic key is not modelled

> "make sure the magic key will be used correctly (h in the start of words or after consonants
> and v otherwise)"

`h|v` on `alpha1` is an adaptive key (urob's zmk-adaptive-key): it produces **h at the start of a
word or after a consonant, and v after a vowel**. Its mirror `v|h` on `alpha2` produces v after a
consonant or at a word start, and h after a vowel. So a human types:

| Wanted | After | Keys |
|---|---|---|
| `h` | word start, consonant | `h|v` on alpha1 (one key) |
| `h` | vowel | `alpha2` thumb, then `v|h` |
| `v` | vowel | `h|v` on alpha1 (one key) |
| `v` | word start, consonant | `alpha2` thumb, then `v|h` |

"Vowel" here is what the firmware's adaptive key treats as one — `a e i o u` and the accented
vowels; read `~/projects/keyboards/src/` (the adaptive-key definitions) rather than assuming.
Space, punctuation and Enter reset to "word start". In the video, `ever` (cold open) and `video`
(Obsidian) show a `v` with no visible source: the resolver found no plain key and fell through
to the `v` combo or lit nothing legible.

### 1.3 Digits are drawn as combos, and the NUMBERS layer sticks

Digits come off the **numbers layer**, held on the space thumb (`{ t: space, h: numbers }`),
never combos — `SCRIPT.md` already says so and `rehearsal-feed.py` claims to emulate it. Two
things went wrong on camera:

- In the cold open (`:21`, ~0:14) the HUD drew **`COMBO 1`** pills over the vim layer: the hold
  was not applied at all there.
- In VS Code (`// bit 1 of the code`, ~4:20) the banner switched to **NUMBERS** for the `1`
  and then **stayed on NUMBERS while `of the cod` was typed** — the restore never fired, so the
  letters were drawn on the wrong drawer.

### 1.4 The LED writes never show while the codes are set (beat 3)

> "the agent tried … to show the keyboard leds with a journalctl command, but it did not work"

`seg3` opens a second Ghostty **tab** for `journalctl … | grep -e decision -e led`, switches
back to tab 1 for the `set` commands, and returns to tab 2 only to `Ctrl-C` it. So the
`led write` lines are on camera for about one second, at the very end, as a dump — never beside
the `set` that caused them. Three more problems in that tab, all visible in the frames:

- The new tab runs the **user's default shell**, not the demo Bash: its prompt shows
  `…/showcase/demo-go main ● ?` (git branch and status), because `-e /bin/bash --rcfile …` on
  the command line only applies to the first surface. Tabs and splits use the config's
  `command`, which `env/ghostty-demo.conf` does not set.
- `-o cat` does not shorten anything: the daemon logs through `slog.NewTextHandler`, so every
  line starts with `time=2026-09-20T16:11:35.488-03:00 level=INFO msg=` and wraps twice at the
  demo font.
- There is no `zmk-vim-mode` subcommand that streams the log (`main.go` has `daemon set status
  devices install uninstall doctor atspi-watch hid-scan`); the journal is the right source.

### 1.5 Smaller things this review found

- The Diamond photo lands late in take 1 (imv cold start) — known; `dub.py` keeps the tail.
- HUD device caption differs between takes (*Diamond* vs *ZMK Project Diamond*): the rehearsal
  feed picked the USB and the BLE endpoint on different runs. Cosmetic; pick one (the config's
  `title:` overrides it).
- Everything else in `TAKE-2-PLAN.md`'s list is fixed in this cut: hook first, no dead air,
  legible at 1.25×, no waybar clock, `(rehearsal)` gone, beat 8's commands typed on camera,
  doctor with `~` paths, ten takes green.

### 1.6 The opening contains a previous attempt

The last rerun began recording while an old demo Ghostty window was still visible on workspace 8.
`record.py` parked there, then `rehearse.py 0` opened another Ghostty a few seconds later. The
old shell was therefore on camera during pre-roll; pause tightening preserved a short piece of
that head. `prepare.sh` used to reset the editors but did not clear this terminal.

**Required:** `prepare.sh` clears the demo Ghostty from workspace 8. `record.py` repeats that
cleanup before every take and refuses to start if a stale Ghostty remains. Do not begin capture
until the workspace and the first-frame preview are clean.

---

## 2 · Why the HUD draws combos, and where to fix it

Nothing is wrong with the HUD. It is drawing exactly what it is told.

- `rehearsal-feed.py` reads the keys ydotool injects (evdev) and sends the pages a **character
  event** per key (`{"kind":"key","chars":"k",…}`), plus, for arrows and digits only, a
  `{"kind":"layers","ids":[…]}` message before the key and a restore after it.
- `hud/hud.js` (zmk-layer-hud) resolves a character **on the live layer stack the keyboard
  reported** — `resolveOnStack()` → `findOnLayer()`, which tries the layer's plain keys first
  and then **its combos**. The keyboard is on `alpha1` (or a vim layer), `alpha2` is not on the
  stack, so `k` is found only as the `alpha1` combo `[1,2]` — and drawn as one, correctly.
  The one-shot alpha2 attribution that exists in `hud.js` (`activatorsOf(ex.alpha2)`, around
  line 545) belongs to the *inferred* path, used when no keyboard stack is known; the rehearsal
  runs *live*, so it never applies.

So the fix is a **typist model in `rehearsal-feed.py`**, in `InjectedKeys`: before emitting a
character, decide how the Diamond's owner would have typed it and emit that.

| Character | Emit |
|---|---|
| plain alpha1 letter, `,` `.` space | as today |
| alpha2 letter (`q k y z x w j`, `'`, `_`, accents) | alpha2 thumb press+release; `layers` = keyboard's stack + alpha2 id; the character; restore |
| `h` / `v` | apply the magic table in §1.2 using the previous typed character; alpha1 `h|v` is a plain key on the stack; the alpha2 case goes through the row above |
| uppercase letter | sticky-shift thumb tap, then the letter (alpha1) — or alpha2 thumb + shift → `shifted2` for alpha2 letters; check how `shifted1`/`shifted2` are entered in `keyboards/src` and mirror it |
| digit | as today (numbers hold), but **layers = keyboard's stack + numbers id**, not numbers alone, and a restore that actually fires (§ below) |
| arrows, Home/End, Page | as today (nav hold), same restore fix |
| Enter, Tab, Esc, `:` `;` `/` `$` `0` | as today; they are combos or single keys on the real board |

Where the pieces are:

- `InjectedKeys.hold_layers()` / `restore_layers()` — extend to alpha2 (one-shot) and to a
  stack-preserving hold. The activator positions come from the keymap message the feed already
  caches (`hub.cache["keymap"]`, built by `host/keymap.py`; `activators()` there lists `{layer,
  idx, kind}` and `hud.js` reads them as `state.data.activators`). To flash the thumb itself,
  emit `{"kind":"press","pos":N}` / `release` with the **ZMK position** of that drawer index
  (`positions:` in `~/.config/zmk-layer-hud/config.yaml` is the drawer→ZMK map; `hudfeed.py`
  `_check_position` shows how it is used), or, simpler, rely on the pages' activator highlight
  once the alpha2 layer is in the `layers` message.
- **The restore.** `true_layers()` in `main()` reads `reader._streams[].decoder.layers` and
  returns `None` until a stream has reported — and `restore_layers()` emits nothing on `None`.
  A same-valued heartbeat from the keyboard re-asserts nothing, so a missed restore sticks
  until the next real layer change. That is the NUMBERS-through-`of the cod` frame. Cache the
  last `layers` message that went through the hub instead (`hub.cache` already keeps the
  keymap; keep `layers` too) and restore from that; if it is still unknown, restore to the
  base layer, never to nothing.
- **The missing hold in the cold open.** Hypothesis: the keymap message had not arrived when
  `seg0` started typing (first seconds of the take), so `drawer_ids("numbers")` was empty and
  no hold was emitted. Verify with `rehearsal-feed.py --debug` on `rehearse.py 0`; if so, make
  `rehearse.py` wait for the feed to report the keymap before the first keystroke (it already
  waits for the panel).
- **Previous-character tracking** for the magic key lives naturally in `InjectedKeys` (one
  attribute, reset on space/punctuation/Enter/Esc and on any non-character key).
- Add a **unit test** next to `hudfeed_test.py`'s style: feed the evdev events for
  `video ever have the kx` and assert the exact message sequence (thumb, layers, char,
  restore). This is the check that survives a refactor; frames are the check that the video
  is right.

Do not put this logic in `hud.js`: the HUD's job is to draw what the keyboard did, and the
real Diamond will send real alpha2 activations. Only the rehearsal's *fake* keyboard needs a
typist.

---

## 3 · Beat 3: the LED writes beside the commands

Goal on camera: the `set` command and the `led write` lines it causes, **visible together**.

1. **Same shell everywhere.** Make `rehearse.py` write a generated config (`run/ghostty.conf`
   = `env/ghostty-demo.conf` + `command = /bin/bash --noprofile --rcfile <absolute path to
   env/demo.bashrc> -i`) and launch the demo Ghostty with it, so tabs and splits are demo shells
   too. Keep `-e` as well for the first surface.
2. **A split, not a tab.** One maximized window, split down: commands on top, the tail below,
   both on camera the whole time. Ghostty's default binds are `ctrl+shift+e` (split down) and
   `ctrl+shift+o` (split right), focus moves with `ctrl+alt+↑/↓`; **confirm on the box** with
   `ghostty +show-config --default | grep -i split` before hard-coding them in `KEYCODES`.
3. **Short lines.** The prefix is fixed-width (`time=<RFC3339 with ms and offset> level=INFO
   msg=`, 50 characters on this box — count it once with `journalctl --user -u zmk-vim-mode
   -o cat -n 1`), so `cut -c 51-` strips it without a single quote character (ydotool drops
   quotes, see `HANDOFF.md`). Filter to the LED writes only:

   ```bash
   journalctl --user -u zmk-vim-mode -f -o cat | grep -e led -e decision | cut -c 51-
   ```

   Expected per `set`: one `"decision" mode=… code=… reason=override` line and one `"led
   write" dev=… code=… reason=transition` line per endpoint (two: USB and Bluetooth). If the
   quoted `"led write"` in the message bothers on camera, add `| tr -d \"` — still no quotes.
4. **Order.** Start the tail first, let it print nothing, then run the sets one by one with a
   pause long enough for the narration's `[+…]` cues (`SCRIPT.md` beat 3). The closing `status`
   stays.
5. Update `SCRIPT.md` beat 3 (Screen, Expect, Cut, shot D) and `HANDOFF-TAKE-2.md`'s "tabs, not
   two tiled windows" note when this lands.

---

## 4 · Re-record and verify

Process is unchanged from `HANDOFF-TAKE-2.md` and `SCRIPT.md` *One process, ten beats*:
scale 1.25, `prepare.sh`, take HUD stopped, `ZMK_RECORD_FPS=60 … record.py all`, then
`dub.py` (anchors → render → master → pauses → tighten → check). Before pressing record:

0. Confirm the monitor is 2560×1440 at scale 1.25. Run `prepare.sh`; it closes the old demo
   terminal on workspace 8, resets demo content and editor state, and strips the clutter widgets.
   The shell config hot-reloads; **do not run `omarchy-restart-shell`** as part of this capture
   (QuickShell has a recurring crash on restart). Confirm no `com.mitchellh.ghostty` window is
   left on workspace 8. `record.py` repeats this check before every take.
1. Run one segment per rule with `rehearsal-feed.py --debug` and read the feed log before
   recording anything. `rehearse.py 0` types `ever`: `e` is a vowel, so its `v` must be the
   alpha1 `h|v` key, no thumb. `rehearse.py 7` types `video`: word-start `v`, so alpha2 thumb
   then `v|h`. `rehearse.py 5` types `/Kana` and `keyboard`: `K` and `k` are alpha2 letters,
   so thumb (plus sticky shift for `K`) then the key, never the `[1,2]` combo. `rehearse.py 4`
   types `bit 0 of the code`: NUMBERS for the `0` only, INSERT back before `of`.
2. Frame check, not log check: `ffmpeg -i run/take5.mp4 -vf "fps=2,crop=640:560:1920:40"
   frames/%04d.jpg`, tile them, and look at every COMBO pill. Allowed pills: `⏎`, `⇥`, the vim
   layer's own combos (`⎋`, `` ` ``, `vim mode`), `:`/`;`. **Any letter or digit in a pill is a
   failure.** Every NUMBERS/NAV banner must end with the key that caused it.
3. `dub.py keys N` stacks the typed-keys strip per take; use it to confirm the sequence, then
   `dub.py proof N` on beats 0, 4, 5 and 7 for the finished video.
4. Beat 3: freeze a frame during `set insert` — the top pane shows the command, the bottom pane
   the `decision` and two `led write` lines, no `time=` prefix, demo prompt in both panes.
5. The first narration cue is the exact introduction requested above. It begins at `+5.0`, when
   the fresh Neovim shot is on screen; do not move it to `+0.0`, which would speak over setup
   frames before the demo window appears.
6. Watch the system journal for `Quickshell has crashed`, `SIGSEGV`, or `dumped core` from before
   capture through the last take. No shell restart is expected during recording. A crash dialog
   or stale terminal in any take invalidates that capture; clean the environment and record again.
7. Inspect the tightened master's first two seconds separately. There must be no stale shell,
   editor buffer, or image from a previous attempt before the spoken introduction.

Definition of done: the four frame checks above pass on the tightened master, `dub.py check`
reports the first cue audible at all ten expected cue points and loudness in range, the opening
contains only the fresh cold-open shot under the requested introduction, and nothing in
`TAKE-2-PLAN.md` §1 has come back.

---

## 5 · What makes it a killer video (do not lose these while fixing)

- The HUD **is** the product for half the audience. A keyboard person spots a fake combo or a
  stuck layer in one frame; after this fix, what lights is exactly what the owner's fingers
  would do, and the video can say so ("this panel is not a mock-up").
- The first flip stays at 0:00. Nothing goes in front of the cold open.
- The new spoken introduction is over the first clean cold-open shot, not an old terminal or a
  title card.
- One full tour (Neovim); the other editors one beat each; no dead air (`dub.py tighten`).
- The daemon's reason on camera where RAW and OFF look alike (beats 3 and 8): typed `status`
  lines, now with the LED writes beside them.
- 60 fps, scale 1.25, `press_ms` 500 for the takes (revert to 320 after), bar widgets stripped.
- Human read against the cut for the deliverable; the TTS is scratch (`YOUTUBE.md` for the rest).

## 6 · Files this touches

| File | Change |
|---|---|
| `showcase/rehearsal-feed.py` | the typist model (§2): alpha2 one-shot, magic key, stack-preserving holds, working restore, readiness |
| `showcase/rehearse.py` | wait for the feed's keymap before typing; beat 3 as a split with the generated Ghostty config; split keycodes |
| `showcase/record.py` | close stale workspace-8 Ghostty surfaces before each recorded take |
| `showcase/prepare.sh` | clear the old demo terminal and hot-reload the temporary bar layout without restarting QuickShell |
| `showcase/env/ghostty-demo.conf` → `run/ghostty.conf` | generated with `command =` so tabs/splits are demo shells |
| `showcase/SCRIPT.md` | exact spoken opening, cue timing, beat 3 Screen/Expect/Cut, and the alpha2/magic typing rules |
| `showcase/dub.py` | verify cue points against silence in the tightened output and compute density over kept footage |
| `showcase/HANDOFF-TAKE-2.md` | supersede the "tabs" note |
| `showcase/rehearsal_feed_test.py` | verify alpha2/magic, number-layer restores and uppercase thumb flashes |
