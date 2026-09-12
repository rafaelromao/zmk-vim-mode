# zmk-vim-mode

Keeps a ZMK keyboard's layers in sync with the editor's real vim state, so the
keyboard is in NORMAL when the editor is in normal mode, in INSERT when you are
typing, and out of vim layers entirely when keys must pass through untouched.

Three parts, one repository:

| Part | Path | Role |
|---|---|---|
| Host daemon | `cmd/`, `internal/` | decides the state, writes it to the keyboard |
| Neovim plugin | `lua/`, `plugin/` | reports Neovim's effective state over a unix socket |
| ZMK module | `firmware/`, `zephyr/` | decodes the state and switches layers |

## How it works

### The channel

A HID keyboard has an *output* report: a byte the host sends **to** the
keyboard, normally to light the Caps Lock and Num Lock lamps. It is the only
standard host-to-keyboard channel that works identically over USB and Bluetooth
with stock ZMK, needs no pairing or custom protocol, and requires nothing of
the firmware beyond `CONFIG_ZMK_HID_INDICATORS=y`. This project uses it as a
data bus rather than as lamp control.

ZMK declares five indicator bits. You can confirm this in your own keyboard's
HID report descriptor, where `19 01 29 05` means "usage minimum NumLock,
usage maximum Kana":

```
05 08  19 01  29 05  75 01  95 05  91 02
│      │      │      │      │      └─ output: data, variable, absolute
│      │      │      │      └──────── five of them
│      │      │      └─────────────── one bit each
│      │      └────────────────────── usage max = Kana      (0x05)
│      └───────────────────────────── usage min = Num Lock  (0x01)
└──────────────────────────────────── usage page = LED
```

Three of those five are free, assuming the operating system never *sets* **Compose**,
**Kana** or **Scroll Lock**. Num Lock and Caps Lock are deliberately
left alone, because the OS owns them — a stray lock keypress would otherwise
change your editor state. Those three free bits are read as one 3-bit number.

### The codes on the wire

| code | state | Scroll `0x04` | Kana `0x10` | Compose `0x08` | byte | keyboard |
|---|---|---|---|---|---|---|
| 0 | off | · | · | · | `0x00` | no vim layers |
| 1 | normal | · | · | ● | `0x08` | `VIM_NORMAL` |
| 2 | insert | · | ● | · | `0x10` | `VIM_INSERT` (Neovim's Replace maps here too) |
| 3 | visual | · | ● | ● | `0x18` | `VIM_NORMAL` + `VIM_VISUAL` |
| 4 | legacy | ● | · | · | `0x04` | vim-like app with no mode feed; the keyboard infers modes itself |
| 5 | cmdline | ● | · | ● | `0x0c` | `VIM_CMDLINE` |
| 6 | raw | ● | ● | · | `0x14` | no vim layers: keys pass through untouched |
| 7 | legacy silent | ● | ● | ● | `0x1c` | same state as 4, re-asserted without re-injecting Esc |

The lamps stay dark: unless the keymap has a `zmk,indicator-leds` node, nothing
physically lights up. It is a silent side channel that happens to travel on the
LED wire.

### End to end, pressing `i` in Neovim

```
nvim ModeChanged ──► plugin ──► unix socket ──► daemon
                                                  │ decides code 2 (insert)
                                                  ▼
                              write [0x01, 0x10] to /dev/hidrawN
                                                  │  report id, LED byte
                     kernel ──► USB SET_REPORT  /  BLE GATT write
                                                  ▼
                     ZMK zmk_hid_indicators_process_report()
                                                  ▼
                       event: zmk_hid_indicators_changed
                                                  ▼
                       this module: decode 0x10 → code 2
                                                  ▼
             deactivate the managed layers, activate VIM_INSERT
```

The firmware side does no scanning and types no keys: it tests three bits,
rebuilds the integer, and if it differs from the last one, switches the layer
set. That is a bitmask change, so it is effectively instant.

Writes go to **every** matching endpoint. A keyboard commonly enumerates on USB
and Bluetooth at the same time, and ZMK stores the indicator byte per endpoint
while only the selected one raises the event, so writing just one is a coin
flip.

### Raw, the state focus watchers cannot produce

**Raw** is reported for plugin UI buffers whose letters are commands
(dashboard, file explorer, lazy, mason, trouble), for terminal-job mode, and
while a `<leader>` sequence is pending. Without it, the keyboard's NORMAL layer
would remap those letters and the UI would be unusable. It is deliberately
distinct from insert: insert keeps the keyboard's local inference alive, so Esc
would flip it to normal — wrong when Esc belongs to a terminal job or a picker.

### Two timing rules that make it survive real use

The keyboard keeps its own local inference and the host only *corrects* it.
Two rules in the firmware keep that hybrid honest:

- **OFF hold-off, 60 ms.** When the kernel sends its own LED report — you
  pressed Num Lock, or a compositor pushed lock state — it sends the whole
  byte, zeroing our bits. Code 0 is therefore applied only after it has been
  stable for 60 ms. The daemon sees the `EV_LED` echo and rewrites within about
  a millisecond, so the blip never reaches your layers.
- **Local guard, 150 ms.** A host code describes the editor as of an earlier
  keystroke. Type `Esc` then `i` quickly and an in-flight stale code could undo
  a correct local transition, so after any keyboard-driven layer change host
  codes are parked and only the newest is applied. The keyboard wins in motion,
  the host wins at rest.

On Linux the steady state is quiet: typing produces no LED reports at all,
because the kernel's cache holds 0 for Compose and Kana, so a compositor
writing "all five bits" changes nothing and is dropped before any report is
emitted. Re-assertion happens only on a real clobber or a reconnect.

## The keyboard side

### The least you need

Four layers and one node. The layers may be entirely `&trans` to begin with —
a vim layer is useful because you *put* something on it, but nothing breaks
while it is empty, and `&to` leaves the base layer showing through.

```c
#include <dt-bindings/zmk/hid_usage.h>

#define BASE        0
#define VIM_NORMAL  1
#define VIM_VISUAL  2
#define VIM_INSERT  3
#define VIM_CMDLINE 4

/ {
    vim_sync {
        compatible = "zmk,hid-indicator-code-listener";
        indicators = <HID_USAGE_LED_COMPOSE HID_USAGE_LED_KANA HID_USAGE_LED_SCROLL_LOCK>;
        managed-layers = <VIM_NORMAL VIM_VISUAL VIM_INSERT VIM_CMDLINE>;

        normal        { code = <1>; layers = <VIM_NORMAL>; };
        insert        { code = <2>; layers = <VIM_INSERT>; };
        visual        { code = <3>; layers = <VIM_NORMAL VIM_VISUAL>; };
        legacy        { code = <4>; layers = <VIM_NORMAL>; bindings = <&kp ESC>; };
        cmdline       { code = <5>; layers = <VIM_CMDLINE>; };
        raw           { code = <6>; };
        legacy_silent { code = <7>; layers = <VIM_NORMAL>; };
        // code 0 is implicit: every managed layer off.
    };
};
```

With an editor that reports its mode — Neovim, or VSCode through
vscode-neovim, or Obsidian — that is the whole keymap side. The host names the
mode and the module switches layers; the keyboard never has to guess.

### Inferring modes on the keyboard

Codes 4 and 7 say only *"a vim-like editor has focus"*: the mode is unknown,
because the editor has no plugin (IntelliJ, vim over SSH), or because you
entered vim mode by hand. Then the keyboard has to follow the mode itself, by
watching the keys that change it.

These are the transitions worth implementing. `^C` behaves as `Esc`
throughout; `I A O S C` are the shifted forms of the letters beside them.

| In layer | Key | Goes to | Why |
|---|---|---|---|
| NORMAL | `i` `a` `o` `s` `I` `A` `O` `S` `C` | INSERT | the insert commands |
| NORMAL | `c` | INSERT | see the caveat below |
| NORMAL | `R` | INSERT | replace types like insert |
| NORMAL | `v` `V` `^V` | VISUAL | the three visual modes |
| NORMAL | `:` `/` `?` | CMDLINE | command line and search |
| INSERT | `Esc` | NORMAL | |
| VISUAL | `Esc` | NORMAL | |
| VISUAL | `v` | NORMAL | pressing `v` again leaves visual |
| VISUAL | `c` `s` | INSERT | change the selection |
| VISUAL | `d` `x` `y` `p` `J` `=` `~` `u` | NORMAL | operators that consume the selection |
| VISUAL | `:` | CMDLINE | `:'<,'>` |
| CMDLINE | `Enter` `Esc` | NORMAL | the line is submitted or abandoned |

Nothing tracks leaving vim altogether (`ZZ`, `:q`, closing the editor) — that
is the host's job, and it sends code 0.

### A sample implementation

One macro helper covers nearly every row of that table: tap the key, then
switch layer. ZMK's `&to` is the whole trick — it activates one layer and drops
every other except the base, which is exactly how vim modes behave. Visual is
the one exception, and uses `&tog` so it stacks on normal instead of replacing
it.

```c
// Tap KEY, then make LAYER the only active layer.
#define VIM_KEY(NAME, KEY, LAYER) \
    NAME: NAME { \
        compatible = "zmk,behavior-macro"; \
        #binding-cells = <0>; \
        bindings = <&kp KEY &to LAYER>; \
    };

/ {
    macros {
        VIM_KEY(vim_i,     I,     VIM_INSERT)
        VIM_KEY(vim_a,     A,     VIM_INSERT)
        VIM_KEY(vim_o,     O,     VIM_INSERT)
        VIM_KEY(vim_s,     S,     VIM_INSERT)
        VIM_KEY(vim_c,     C,     VIM_INSERT)
        VIM_KEY(vim_r,     R,     VIM_INSERT)
        VIM_KEY(vim_colon, COLON, VIM_CMDLINE)
        VIM_KEY(vim_slash, SLASH, VIM_CMDLINE)
        VIM_KEY(vim_esc,   ESC,   VIM_NORMAL)
        VIM_KEY(vim_enter, RET,   VIM_NORMAL)
        VIM_KEY(vim_d,     D,     VIM_NORMAL)   // visual-mode operators
        VIM_KEY(vim_y,     Y,     VIM_NORMAL)
        VIM_KEY(vim_p,     P,     VIM_NORMAL)
        VIM_KEY(vim_x,     X,     VIM_NORMAL)

        // Visual sits *on top of* normal, the way code 3 does, so it toggles
        // rather than replaces. The same macro serves both rows of the table:
        // `v` in normal turns VISUAL on, `v` in visual turns it off again.
        vim_v: vim_v {
            compatible = "zmk,behavior-macro";
            #binding-cells = <0>;
            bindings = <&kp V &tog VIM_VISUAL>;
        };
    };
};
```

Then place them. Only the keys that change mode need an entry; everything else
is `&trans`, so your base layout shows through — or your own vim bindings, if
the point of the NORMAL layer is to move `hjkl` somewhere better.

```c
vim_normal_layer {
    display-name = "NORMAL";
    bindings = <
        // ... &trans for the keys that do not change mode ...
        &vim_i      &vim_a     &vim_o     &vim_s    &vim_c
        &vim_r      &vim_v     &vim_colon &vim_slash
    >;
};

vim_insert_layer {
    display-name = "INSERT";
    bindings = <
        // all &trans except:
        &vim_esc
    >;
};

vim_visual_layer {
    display-name = "VISUAL";
    bindings = <
        // sits over VIM_NORMAL, so it only needs the keys that leave visual;
        // motions fall through to the layer underneath:
        &vim_esc    &vim_v     &vim_c     &vim_d    &vim_y    &vim_p   &vim_x
    >;
};

vim_cmdline_layer {
    display-name = "CMDLINE";
    bindings = <
        // all &trans except:
        &vim_esc    &vim_enter
    >;
};
```

Shifted variants (`I`, `A`, `O`, `C`, `V`) are the same macros behind a
mod-morph, or simply the same key: `&vim_i` with Shift held already sends `I`
and lands in INSERT, which is the correct outcome.

Two things this simple version gets wrong, both survivable:

- **`c{motion}`** — `cw` switches to INSERT on the `c`, so the `w` is typed
  with the NORMAL layer already gone. Harmless when NORMAL leaves letters
  alone; when NORMAL remaps letters, the motion key is the wrong one. The fix
  is a short-lived operator-pending layer — see
  [docs/keyboards-repo.md](docs/keyboards-repo.md) for one.
- **counts and registers** — `3x`, `"ayy`: the digits and the register name
  pass through NORMAL unchanged, which is right, but `x` in `3x` still returns
  to NORMAL, which it already was. No harm.

Anything the keyboard gets wrong here is corrected by the host within a
keystroke as soon as a reporting editor is focused: local inference only has
to be good enough for the editors that cannot speak.

### Why the layers are worth having

Switching layers to follow the editor would be a curiosity if the layers only
mirrored your base layout. The point is that they do not have to.

`hjkl` sits on QWERTY's home row by accident of history. Move to any modern
alternative and those four letters scatter. Gallium, the layout in the example
below:

```
b l d c v   j y o u ,
n r t s g   p h a e i
x q m w z   k f ' ; .
```

`h` keeps the right index home position, but `j` and `k` are stacked on the
index's inner column — one reach up, one reach down, same finger — and `l` is
on the **left hand**, middle finger, top row. Cursor movement becomes a
one-finger stretch plus a hand alternation. Colemak, Dvorak and Graphite each
scatter it differently; none of them keeps the row.

The usual answers are all bad. Remap vim and you fight every plugin, tutorial
and muscle memory that assumes the defaults, on every machine you ever ssh
into. Learn the scattered positions and you have made your best layout worse
at the thing you do most. Give up on motions and you are not really using vim.

A vim layer removes the compromise. While the editor is in normal mode the
keyboard is not typing letters at all — it is issuing commands — so that layer
can be an entirely different map, chosen for how vim is actually used:

- **motions on the home row**, `h j k l` under your strongest fingers,
  whatever your base layout does with those letters;
- **operators under the other hand**, so `d`, `y`, `c`, `v` are one comfortable
  key each instead of wherever the alphabet left them;
- **two-key commands as one key** — `dd`, `yy`, `gg` are macros, and `^D`/`^U`
  need no modifier;
- **punctuation that matters in normal mode gets prime keys** — `:` and `/`
  are far more frequent than `;` or `'` while you are in normal mode, so they
  take the good positions.

None of it costs you anything while typing, because the layer is gone the
moment the editor goes back to insert: `VIM_INSERT` is transparent, so your
base layout shows through untouched. That is the trade this project exists to
make — a dedicated command layout that appears exactly when the editor is
expecting commands, and disappears exactly when it is not.

**And vim itself needs no configuration.** The keyboard sends real `h`, `j`,
`k`, `l` keycodes — the layer decides which physical key produces them, not
what the editor does with them. So there is no `noremap` in your config,
nothing to keep in sync between machines, nothing that breaks when a plugin
binds `gj` or expects `dw` to work, and nothing to install on the server you
ssh into. Stock vim, stock plugins, a keyboard that speaks their language.

One ordering rule makes it behave: **keep the vim layers below your other
layers**. Layer priority in ZMK is numeric, so with `NAV` and `SYM` above them,
holding a nav key still works while vim layers are active. Put them above and
the vim layer would shadow everything you hold.

### A complete 34-key keymap

A 3×5+2 board (Ferris Sweep, Cradio, Corne without the outer columns). Gallium
base with home-row mods, and the four vim layers. It is written to compile as
it stands — drop it in as your `.keymap`, swap the base layer for whatever you
actually type on, and the vim layers need no changes at all: they name
keycodes, not positions on your alpha layout.

```c
#include <behaviors.dtsi>
#include <dt-bindings/zmk/keys.h>
#include <dt-bindings/zmk/hid_usage.h>

#define BASE        0
#define VIM_NORMAL  1
#define VIM_VISUAL  2
#define VIM_INSERT  3
#define VIM_CMDLINE 4
#define NAV         5
#define SYM         6

// Tap KEY, then make LAYER the only active layer.
#define VIM_KEY(NAME, KEY, LAYER) \
    NAME: NAME { \
        compatible = "zmk,behavior-macro"; \
        #binding-cells = <0>; \
        bindings = <&kp KEY &to LAYER>; \
    };

// Tap KEY twice, staying where we are: dd, yy, gg.
#define VIM_PAIR(NAME, KEY) \
    NAME: NAME { \
        compatible = "zmk,behavior-macro"; \
        #binding-cells = <0>; \
        bindings = <&kp KEY &kp KEY>; \
    };

/ {
    vim_sync {
        compatible = "zmk,hid-indicator-code-listener";
        indicators = <HID_USAGE_LED_COMPOSE HID_USAGE_LED_KANA HID_USAGE_LED_SCROLL_LOCK>;
        managed-layers = <VIM_NORMAL VIM_VISUAL VIM_INSERT VIM_CMDLINE>;

        normal        { code = <1>; layers = <VIM_NORMAL>; };
        insert        { code = <2>; layers = <VIM_INSERT>; };
        visual        { code = <3>; layers = <VIM_NORMAL VIM_VISUAL>; };
        legacy        { code = <4>; layers = <VIM_NORMAL>; bindings = <&kp ESC>; };
        cmdline       { code = <5>; layers = <VIM_CMDLINE>; };
        raw           { code = <6>; };
        legacy_silent { code = <7>; layers = <VIM_NORMAL>; };
    };

    behaviors {
        // Esc that also returns to NORMAL, still holdable for the nav layer.
        // A hold-tap's bindings are phandles, so the tap must be a behavior
        // that takes no parameter of its own -- hence the macro.
        esc_nav: esc_nav {
            compatible = "zmk,behavior-hold-tap";
            #binding-cells = <2>;
            flavor = "tap-preferred";
            tapping-term-ms = <200>;
            bindings = <&mo>, <&vim_esc>;
        };
    };

    macros {
        VIM_KEY(vim_i,     I,     VIM_INSERT)
        VIM_KEY(vim_a,     A,     VIM_INSERT)
        VIM_KEY(vim_o,     O,     VIM_INSERT)
        VIM_KEY(vim_c,     C,     VIM_INSERT)
        VIM_KEY(vim_x,     X,     VIM_NORMAL)
        VIM_KEY(vim_p,     P,     VIM_NORMAL)
        VIM_KEY(vim_d,     D,     VIM_NORMAL)
        VIM_KEY(vim_y,     Y,     VIM_NORMAL)
        VIM_KEY(vim_esc,   ESC,   VIM_NORMAL)
        VIM_KEY(vim_enter, RET,   VIM_NORMAL)
        VIM_KEY(vim_colon, COLON, VIM_CMDLINE)
        VIM_KEY(vim_slash, FSLH,  VIM_CMDLINE)

        VIM_PAIR(vim_dd, D)
        VIM_PAIR(vim_yy, Y)
        VIM_PAIR(vim_gg, G)

        // Visual stacks on normal, the way code 3 does, so it toggles rather
        // than replaces -- and the same key leaves visual again.
        vim_v: vim_v {
            compatible = "zmk,behavior-macro";
            #binding-cells = <0>;
            bindings = <&kp V &tog VIM_VISUAL>;
        };
    };

    keymap {
        compatible = "zmk,keymap";

        // Gallium. Note where h, j, k and l fall: h on the right index home,
        // j and k stacked on the index's inner column, l on the other hand.
        base_layer {
            display-name = "BASE";
            bindings = <
   &kp B        &kp L        &kp D         &kp C         &kp V        &kp J      &kp Y          &kp O         &kp U        &kp COMMA
   &mt LGUI N   &mt LALT R   &mt LCTRL T   &mt LSHFT S   &kp G        &kp P      &mt RSHFT H    &mt RCTRL A   &mt RALT E   &mt RGUI I
   &kp X        &kp Q        &kp M         &kp W         &kp Z        &kp K      &kp F          &kp SQT       &kp SEMI     &kp DOT
                                           &lt NAV ESC   &kp SPACE    &kp RET    &lt SYM BSPC
            >;
        };

        // Commands, not letters: motions on the right home row, operators on
        // the left, ":" and "/" on the pinkies where they are cheap to reach.
        vim_normal_layer {
            display-name = "NORMAL";
            bindings = <
   &kp ESC      &vim_c       &vim_o        &vim_i        &vim_a       &kp LC(U)  &kp W          &kp E         &kp B        &kp DLLR
   &kp LC(R)    &kp U        &vim_v        &vim_dd       &vim_yy      &kp H      &kp J          &kp K         &kp L        &vim_colon
   &kp DOT      &vim_x       &kp P         &vim_slash    &kp N        &kp LC(D)  &vim_gg        &kp LS(G)     &kp CARET    &kp PRCNT
                                           &trans        &trans       &trans     &trans
            >;
        };

        // Sits on top of NORMAL: only the keys that end the selection differ,
        // every motion falls through to the layer underneath.
        vim_visual_layer {
            display-name = "VISUAL";
            bindings = <
   &trans       &vim_c       &trans        &trans        &trans       &trans     &trans         &trans        &trans       &trans
   &trans       &trans       &vim_v        &vim_d        &vim_y       &trans     &trans         &trans        &trans       &vim_colon
   &trans       &vim_x       &vim_p        &trans        &trans       &trans     &trans         &trans        &trans       &trans
                                           &trans        &trans       &trans     &trans
            >;
        };

        // Transparent: your base layout, plus an Esc that goes back to NORMAL.
        vim_insert_layer {
            display-name = "INSERT";
            bindings = <
   &trans       &trans       &trans        &trans        &trans       &trans     &trans         &trans        &trans       &trans
   &trans       &trans       &trans        &trans        &trans       &trans     &trans         &trans        &trans       &trans
   &trans       &trans       &trans        &trans        &trans       &trans     &trans         &trans        &trans       &trans
                                           &esc_nav NAV 0 &trans      &trans     &trans
            >;
        };

        // The command line: Esc abandons it, Enter submits it, both end in NORMAL.
        vim_cmdline_layer {
            display-name = "CMDLINE";
            bindings = <
   &trans       &trans       &trans        &trans        &trans       &trans     &trans         &trans        &trans       &trans
   &trans       &trans       &trans        &trans        &trans       &trans     &trans         &trans        &trans       &trans
   &trans       &trans       &trans        &trans        &trans       &trans     &trans         &trans        &trans       &trans
                                           &esc_nav NAV 0 &trans      &vim_enter &trans
            >;
        };

        // Above the vim layers, so holding it still works while they are active.
        nav_layer {
            display-name = "NAV";
            bindings = <
   &kp TAB      &kp N7       &kp N8        &kp N9        &kp MINUS    &kp HOME   &kp PG_DN      &kp PG_UP     &kp END      &kp DEL
   &kp N0       &kp N4       &kp N5        &kp N6        &kp EQUAL    &kp LEFT   &kp DOWN       &kp UP        &kp RIGHT    &kp BSPC
   &kp GRAVE    &kp N1       &kp N2        &kp N3        &kp BSLH     &kp C_PP   &kp C_VOL_DN   &kp C_VOL_UP  &kp C_MUTE   &kp CAPS
                                           &trans        &trans       &trans     &trans
            >;
        };

        sym_layer {
            display-name = "SYM";
            bindings = <
   &kp EXCL     &kp AT       &kp HASH      &kp DLLR      &kp PRCNT    &kp CARET  &kp AMPS       &kp STAR      &kp LPAR     &kp RPAR
   &kp TILDE    &kp UNDER    &kp PLUS      &kp LBRC      &kp RBRC     &kp PIPE   &kp SQT        &kp DQT       &kp LBKT     &kp RBKT
   &kp F1       &kp F2       &kp F3        &kp F4        &kp F5       &kp F6     &kp F7         &kp F8        &kp F9       &kp F10
                                           &trans        &trans       &trans     &trans
            >;
        };
    };
};
```

Reading it as a vim user: `i a o c` enter insert from the left hand, `v` opens
visual and closes it again, `dd`/`yy`/`gg` are single keys, `:` and `/` open
the command line, and the right hand keeps `h j k l` on home with `w e b` above
and `^D`/`^U` for paging. Everything not listed falls through to the base
layer, so counts, registers and the commands you use once a month still work
exactly as they do in vim.

Compare the two right hands. On the Gallium base, `h j k l` are `h`, an
up-reach, a down-reach and a key on the left hand. On the normal layer they are
index, middle, ring, pinky — and `p`, `a`, `e`, `i` are still exactly where
Gallium puts them the moment you press `i`.

## Install

```bash
make install
```

One command, both platforms. It builds and installs the binary, writes the
user service and starts it, installs the Neovim plugin spec, and sets up
VSCode and Obsidian — each editor is skipped when it is not installed, and an
existing Neovim spec is never touched. On Linux it also installs the udev rule
(the one `sudo` prompt) and enables the accessibility bus; on macOS it signs
the binary and loads the launchd agent. Running it again is how you upgrade:
it restarts the service, so it never leaves the old binary running.

Four things it cannot do for you:

- **flash the firmware module** — see [docs/keyboards-repo.md](docs/keyboards-repo.md);
- **restart the editors**, so they load their new plugins;
- **`set -g focus-events on`** in `~/.tmux.conf`, or Neovim never sees
  `FocusLost` inside tmux;
- on macOS, **grant the two permissions** below.

Then check everything:

```bash
zmk-vim-mode doctor
```

### macOS permissions

Two grants, each added by hand under System Settings → Privacy & Security,
both pointing at `~/.local/bin/zmk-vim-mode` (`+`, then ⌘⇧G to type the path):

- **Input Monitoring** — required to open the keyboard's HID device, so
  without it nothing reaches the keyboard at all. `zmk-vim-mode devices` shows
  `writable` once it is in place.
- **Accessibility** — lets the daemon read window titles, which is how tool
  windows are detected (VSCode's terminal, sidebar, panels). Without it only
  the editors' own plugins report.

A background agent is never prompted for either, which is why they have to be
added manually. `zmk-vim-mode doctor` reports what the **daemon** was granted —
the only answer that counts, since macOS judges such a request by the
responsible process, and a CLI run from a terminal is judged on that
terminal's permissions.

Both grants are tied to the binary's code signature, and Go stamps the version
into every build, so each rebuild would void them. `make install` avoids that
by signing with a self-signed certificate when the keychain has one:

```bash
make codesign-cert    # once; asks to allow codesign to use the key, then for your login password
```

Without it the binary is signed ad-hoc and both permissions must be removed
and re-added after every rebuild. `make build` says which of the two it used.

### Editors

| Editor | Mode source | Tool-window focus | Setup |
|---|---|---|---|
| Neovim in a terminal, Neovide | the Neovim plugin | the plugin: `raw` for pickers, the terminal, a pending `<leader>` | `zmk-vim-mode install --nvim` (also done by `make install`) |
| VSCode | the same plugin, inside [vscode-neovim](https://github.com/vscode-neovim/vscode-neovim) | window title `[${focusedView}]` read from Hyprland, a companion extension for quick inputs and non-text editors, the accessibility bus for anything opened with the mouse (Linux) | `zmk-vim-mode install --vscode --atspi`, then [editors/vscode](editors/vscode/README.md) |
| Obsidian | own plugin (CodeMirror vim events) | own plugin (`focusin`) | `zmk-vim-mode install --obsidian`, then [editors/obsidian](editors/obsidian/README.md) |
| IntelliJ, anything else | none: `legacy`, the keyboard infers | — | nothing; `set raw` when a tool window traps you |

Inside an app the sources rank: window title (a focused tool window) →
accessibility bus (focus anywhere but the text editor) → the app's own client
saying `raw` → the best client with a real mode → `legacy`. The title wins
because no client can see focus leave the text editor.

#### Neovim plugin options

Passed as `opts` in the lazy.nvim spec; all have working defaults.

| Option | Default | Meaning |
|---|---|---|
| `socket` | `~/.local/state/zmk-vim-mode/daemon.sock` | daemon socket; `$ZMK_VIM_MODE_SOCKET` also works |
| `terminal_state` | `raw` | what `:terminal` job mode reports — `insert` keeps the vim layers there |
| `leader_raw` | `true` | report `raw` while a `<leader>` sequence is pending (which-key menus) |
| `raw_filetypes` | see `lua/zmk-vim-mode/context.lua` | filetypes whose single letters are plugin commands |
| `raw_exceptions` | `help qf man checkhealth` | filetypes that stay in vim layers despite looking like UI |
| `classify` | `nil` | `function(buf, win, mode) -> state\|nil`, overrides everything |
| `vscode_raw_actions` | palette, Go to File/Line/Symbol, rename | VSCode commands (Lua patterns) that take the keys away from Neovim |
| `vscode_raw_ttl_ms` | `20000` | how long that hint lasts if its close is never observed |
| `debug` | `false` | log every state change with `vim.notify` |

`:ZmkVimMode status` shows the socket, the connection, the state last sent and
whether a leader or VSCode hint is active.

### Following focus through the accessibility bus (Linux)

Two things nothing above can see: a quick input opened with the mouse, and the
exact moment focus returns to the editor. Both are visible on AT-SPI2, the
Linux accessibility bus, which every toolkit reports focus changes to -- when
accessibility is on. `make install` turns it on; `zmk-vim-mode install --atspi`
does it alone, and dropping `--atspi` from `INSTALL_FLAGS` in the Makefile
leaves it off.

It also adds `--force-renderer-accessibility` to `~/.config/code-flags.conf`
(read by Arch's `code` wrapper): Electron builds the accessibility tree of its
web content only with that switch -- the bus flags alone reach GTK and Qt, not
VSCode's DOM. Restart VSCode afterwards.

The daemon then keeps one connection to the bus, registers as a focus
listener and classifies each focused widget of the frontmost VSCode: the
Monaco code editor → the clients decide; anything else (the palette's input,
the terminal, a tree, a rename box, the find widget) → raw. It also tells the
VSCode companion and the embedded Neovim when the editor regained focus, so
their own guesses clear instantly.

What it costs, and why it is opt-in: the daemon sets both `org.a11y.Status`
flags (`IsEnabled`, `ScreenReaderEnabled`) on the session, which is what a
screen reader does -- GTK, Qt and Chromium applications start maintaining
accessibility trees (a little CPU and memory, nothing visible). `install --vscode` already sets
`editor.accessibilitySupport: off` so VSCode does not switch Monaco into
screen-reader mode because of it. Applications read the flag at startup:
restart VSCode after enabling. The daemon reads only roles, labels and HTML
tag/class names of focused widgets -- never text -- and logs labels at debug
only.

`zmk-vim-mode atspi-watch` prints every focus event with the classifier's
verdict; if a VSCode update renames a widget, that is where the new name shows
up. `zmk-vim-mode status` shows `focus : elsewhere (input.input …)` while a
quick input is open. No D-Bus library is involved: `internal/dbus` is a
300-line client for the handful of calls this needs.

### What differs on macOS

- **LED writes** go through IOKit `IOHIDDeviceSetValueMultiple`: all five
  indicators in one call, so one output report carries the whole code and the
  firmware never decodes a half-written one. (`IOHIDDeviceSetReport` is kept as
  a fallback; on its own it was rejected by the system's keyboard driver.)
  Num and Caps Lock keep whatever the host has lit.
- **No clobber to repair.** macOS never writes Compose, Kana or Scroll Lock, so
  a code is re-asserted only when a device appears and after wake
  (`IORegisterForSystemPower`). There is no `EV_LED`-style echo to watch.
- **Frontmost app** is asked of the accessibility system
  (`kAXFocusedApplication`), or of the window list (`CGWindowList`) when that
  permission is missing, and polled ten times a second. Apps are matched by
  bundle identifier: `com.mitchellh.ghostty`, `com.microsoft.VSCode`,
  `md.obsidian`. `NSWorkspace.frontmostApplication` is only a last resort: it
  updates through Cocoa notifications delivered to a main run loop, which a
  daemon does not run, so it answers with whatever was frontmost at startup
  for as long as the process lives.
- **Window titles need the Accessibility permission.** They are read through
  the Accessibility API (`AXUIElement`), not Screen Recording. Grant it and
  VSCode's `[${focusedView}]` marker works exactly as on Linux, tool windows
  included; without it the daemon sees no titles and only the companion
  extension and the embedded Neovim report VSCode's state. `zmk-vim-mode
  doctor` opens the system dialog; the entry is System Settings → Privacy &
  Security → Accessibility → `~/.local/bin/zmk-vim-mode`.
- **No accessibility bus**: AT-SPI2 is Linux-only, so `--atspi` does nothing
  here and a quick input opened with the mouse is not detected.

Everything else is the same: `make install`, `status`, `doctor`.

Why the signature matters, since it is the one thing with no visible cause:
Go's linker leaves an ad-hoc *linker-signed* signature whose identifier is
`a.out`, and TCC cannot hold a grant against that — the daemon can sit in the
permission list with every device open still refused. `make build` re-signs
with a stable identifier; `make codesign-cert` makes that identifier a
certificate, so the grants outlive rebuilds. By hand the certificate is
**Keychain Access** → menu *Keychain Access* → *Certificate Assistant* →
*Create a Certificate*: Name `zmk-vim-mode-dev`, Identity Type *Self Signed
Root*, Certificate Type *Code Signing* (the dialog opens on *SSL Client*).
`security find-identity -v -p codesigning` lists what the keychain has.

### Troubleshooting (macOS)

| Symptom | Cause | Fix |
|---|---|---|
| `devices`: no keyboards | the keyboard serves another host | `zmk-vim-mode hid-scan` lists what this Mac sees; a ZMK keyboard talks to one BLE profile at a time |
| `hid-scan` shows it, `devices` does not | daemon still on the old binary | `make install` (it restarts the agent) |
| `NOT writable`, *not permitted* | Input Monitoring missing for **this** build | remove and re-add `~/.local/bin/zmk-vim-mode`, then `launchctl kickstart -k gui/$UID/dev.rafaelromao.zmk-vim-mode` |
| writes succeed, layers do not move | the keyboard is acting on another endpoint | ZMK keeps indicators per endpoint and only the selected one raises the event: check the keyboard's output (USB vs BLE) |
| `bootstrap`: `5: Input/output error` | the agent is already loaded | `launchctl kickstart -k gui/$UID/dev.rafaelromao.zmk-vim-mode` |
| VSCode's terminal or sidebar keeps the vim layers | no Accessibility permission, so no window titles | add `~/.local/bin/zmk-vim-mode` under Accessibility, then `launchctl kickstart -k gui/$UID/dev.rafaelromao.zmk-vim-mode`; `doctor` reports what the **daemon** was granted, which is the only answer that counts |
| Obsidian reports nothing | the plugin is in the vault but not enabled | Obsidian rewrites its plugin list on exit, so `install --obsidian` cannot enable it while Obsidian runs: quit Obsidian and run it again, or enable *ZMK Vim Mode* in Settings → Community plugins |
| nothing works, unclear why | — | run it in the foreground from a terminal that already has Input Monitoring: `launchctl bootout gui/$UID/dev.rafaelromao.zmk-vim-mode; zmk-vim-mode daemon --log-level debug` |

## Commands

```
zmk-vim-mode daemon [--atspi]   run the daemon (normally via the user service)
zmk-vim-mode status             current decision, frontmost app, widget focus, clients, devices
zmk-vim-mode devices            keyboards the daemon can write to, and the last code sent to each
zmk-vim-mode set <mode>         manual override; repeating the same mode returns to auto
zmk-vim-mode doctor             daemon, devices, permissions, old watchers, and the editor setups
zmk-vim-mode install [flags]    service, Neovim spec, --vscode, --obsidian, --atspi, --udev, --tmux
zmk-vim-mode uninstall          remove the service (config is left alone)
zmk-vim-mode atspi-watch        Linux: accessibility-bus focus events with the classifier's verdict
zmk-vim-mode hid-scan [--all]   macOS: HID keyboards this host sees and the LEDs they expose
zmk-vim-mode version
```

`set` is the escape hatch for anything the daemon cannot detect (an SSH
session, a screen-sharing app). Bind it to a key with
`contrib/hyprland-bind.conf`. `atspi-watch` and `hid-scan` are the two
diagnostics: the first says how a widget was classified, the second whether the
keyboard is on this host at all.

## Development

```bash
make test          # Go, Neovim and firmware-policy suites
make lint          # gofmt + go vet, for this platform and for Linux
make cross         # static binaries for the Omarchy box (linux/amd64, linux/arm64)
```

Linux builds are pure Go and static; macOS needs cgo for IOKit and Cocoa, and
signs the result (see *What differs on macOS*). `make cross` stays CGO-free, so
it can be run from either machine.

`scripts/spike-linux.sh` covers the bring-up checks on Linux: find the
keyboard's device nodes, confirm the firmware exposes the three LEDs, write a
code by hand, and watch the `EV_LED` echoes that reveal a clobber.

The firmware's decode and timing rules live in `firmware/src/code_policy.h`,
which depends on nothing but `stdint`, so they are unit-tested on the host
(`make test-firmware`) rather than only on hardware.

## Licence

MIT. See [LICENSE](LICENSE).
