# Example keymap: 34 keys, Gallium base

A complete keymap for a 3×5+2 board (Ferris Sweep, Cradio, Corne without the
outer columns): Gallium base with home-row mods, and the four vim layers. It is
written to compile as it stands — drop it in as your `.keymap`, swap the base
layer for whatever you actually type on, and the vim layers need no changes at
all: they name keycodes, not positions on your alpha layout.

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

The README's [Keyboard setup](../README.md#keyboard-setup) explains each part:
the `vim_sync` node, the layer order, and the macros that let the keyboard
follow modes on its own.
