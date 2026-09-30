# Worked example: the author's keyboards repo

How [rafaelromao/keyboards](https://github.com/rafaelromao/keyboards), the
author's ZMK config, takes this module in. It is a reference for wiring the
module into a config of your own, not something zmk-vim-mode needs: your keymap
only needs what *Keyboard setup* in the
[README](https://github.com/rafaelromao/zmk-vim-mode#keyboard-setup) describes.
Line references are from the state of that repo when this was last checked
(`68d98586`).

## 1. Add the module

The module is one entry in the default module list, `DEF_MODULES` at
`scripts/zmk.sh:22`:

```bash
DEF_MODULES=(urob/zmk-leader-key urob/zmk-auto-layer urob/zmk-adaptive-key rafaelromao/zmk-layer-morph rafaelromao/zmk-vim-mode rafaelromao/zmk-layer-hud rafaelromao/zmk-os-detection)
```

That line is the whole change: there is no `git submodule add` to run. For
every listed module that has no `zephyr/module.yml` yet, `zmk.sh` runs
`git submodule add -f git@github.com:<module> modules/<module>` itself
(`scripts/zmk.sh:203-214`), then hands them all to west as `ZMK_EXTRA_MODULES`
(`:326-328`). The `modules/rafaelromao/zmk-vim-mode` entry in `.gitmodules` is
what that leaves behind. No gitlink is committed with it, so a
`--recurse-submodules` clone does not fetch the module; the first build does,
into `modules/`, which inside the container is the `zmk-modules-cache` volume
(`init.sh:88`) rather than the `modules/` of your checkout.

`ssbb/zmk-listeners` is no longer listed; it went with the `num_lock` node
(section 3).

`CONFIG_ZMK_HID_INDICATORS=y` is already set in twelve `.conf` files, though
not in the Zen dongle's `corneish_zen_dongle.conf`, and the module `select`s it
anyway on any central whose keymap has the node. Peripherals need no change.

## 2. Add the sync node to `zmk/features/vim.dtsi`

Only one new macro (`vim_mode_on_host`, see below); every other host-driven
state is a plain layer set. The node is at `vim.dtsi:32-47`; the include it
needs is at `:8`.

```c
#include <dt-bindings/zmk/hid_usage.h>

/ {
    vim_sync: vim_sync {
        compatible = "zmk,hid-indicator-code-listener";
        indicators = <HID_USAGE_LED_COMPOSE HID_USAGE_LED_KANA HID_USAGE_LED_SCROLL_LOCK>;
        managed-layers = <VIM_NORMAL VIM_VISUAL VIM_CHANGE VIM_INSERT VIM_REPLACE VIM_CMDLINE>;
        off-delay-ms = <100>;
        local-guard-ms = <150>;

        normal        { code = <1>; layers = <VIM_NORMAL>; };
        insert        { code = <2>; layers = <VIM_INSERT>; };
        visual        { code = <3>; layers = <VIM_NORMAL VIM_VISUAL>; };
        legacy        { code = <4>; layers = <VIM_NORMAL>; bindings = <&vim_mode_on_host>; };
        cmdline       { code = <5>; layers = <VIM_CMDLINE>; };
        raw           { code = <6>; };
        legacy_silent { code = <7>; layers = <VIM_NORMAL>; };
        /* code 0 is implicit: every managed layer off. */
    };
};
```

### 2a. Legacy mode is not a temporary fallback

Codes 4 and 7, the enter/leave combos (`cb_vim_mode`, `cb_enter_vim`,
`cb_leave_vim`) and the Hyper+Esc / Meh+Esc notification macros were briefly
kept in a separate `vim_legacy.dtsi`, on the theory that they could be deleted
once every editor reported its mode. They cannot, so they live with everything
else, the codes and macros in `vim.dtsi` and the combos in `combos.dtsi`:

- **manual activation is legacy mode.** The combos enter exactly this state,
  and the notification chords exist so the host agrees with a decision made on
  the keyboard. Remove legacy and the combos go with it.
- **vim over SSH** has no plugin on the remote side: the host recognises it by
  the window title alone and sends code 4.
- a known editor whose client has not connected yet, or is disabled, falls
  back to it too.

Notes on the choices:

- **`VIM_CHANGE` and `VIM_REPLACE` are managed but never activated by a code.**
  Both are entered through the sticky `&ntsl`
  (`zmk/definitions/config.dtsi:192-199`); a layer the module activated would be
  switched off again by the next sticky release, because ZMK layer state is a
  bitmask, not a refcount. Listing them under `managed-layers` still lets any
  host code clear them, which is what you want when leaving vim.
- **Neovim's Replace maps to code 2 (INSERT), not to `VIM_REPLACE`.** On the
  keyboard `VIM_REPLACE` is the "next key is a literal" layer behind
  `r<char>`/`q<reg>`/`@<reg>`, not Neovim's `R` mode. The all-`&trans`
  `VIM_INSERT` layer with its Esc handling is the right shape for `R`.
- **Code 4 must not reuse `&vim_mode_on`.** That macro is a layer-morph whose
  "already in vim → `&none`" branch looks idempotent, but the listener clears
  every managed layer *before* invoking a code's bindings, so from code 4 the
  guard can never fire: it would always take the notify branch and send
  Esc **and** Hyper+Esc. Since the host binds Hyper+Esc to
  `zmk-vim-mode set legacy --sticky`, the keyboard would then cancel the very
  override the host had just set. Code 4 therefore selects `VIM_NORMAL`
  declaratively and binds a plain `&kp ESC` (`&vim_mode_on_host`);
  `&vim_mode_on` stays for the combos, where nothing has pre-cleared the layers
  and the guard does work. Code 7 is the same state with no binding at all, so
  a re-assert after a reconnect or a clobber never re-types Esc.
- **There is no `VIM_LEADER` layer any more.** It was an all-`&trans` layer
  entered on `<space>` so the keys after a leader reached the editor untouched.
  The plugin now reports a pending leader as `raw`, which does the same from the
  host, and legacy apps never had leader tracking. Removing it renumbered every
  layer above it in `config.dtsi`.
- **`tc_cancel` no longer notifies the host.** It used `&vim_mode_off`, whose
  Meh+Esc is now bound to `set off --sticky`; a panic key that toggles a sticky
  override on every press is wrong, so it clears the vim layers locally instead:
  it runs `&mc_base_reset`, which starts with `&vim_off`
  (`zmk/features/smart.dtsi:21`, `:44`).
- **Code 6 (raw) lists no layers.** Keys reach the host untouched. Bind a tool
  layer here later if you want one.

## 3. Remove the num-lock listener (recommended)

Done on keyboards `main`: a comment at `zmk/features/vim.dtsi:22-23` is all
that is left of the node that mapped NUM LOCK to vim mode:

```c
        num_lock {
            indicator = <HID_USAGE_LED_NUM_LOCK>;
            bindings = <&vim_mode_on &vim_off>;
        };
```

If your config has one, delete that node (and the surrounding
`hid_listeners`/`zmk,hid-listeners` block if nothing else uses it), and drop
`ssbb/zmk-listeners` from your module list, as `scripts/zmk.sh:22` has. Reason:
on Linux, every libinput LED push carries the kernel's real Num Lock state, so
if `input:numlock_by_default` is ever true the listener re-fires `&vim_mode_on`
and injects an Esc. Manual entry into vim mode remains available through:

- the `cb_vim_mode` combo (`zmk/features/combos.dtsi:52`),
- `cb_enter_vim` / `cb_leave_vim` on the MACROS layer (`combos.dtsi:256-257`),
- `zmk-vim-mode set legacy` from the host.

There is deliberately no `&to VIM_NORMAL` key on the TOGGLES layer; that layer's
`&kp KP_NUM` (`zmk/definitions/keymap.dtsi:135`) used to be the manual toggle,
and is now just an OS num-lock key.

Keep the node only if you use these keyboards on a machine that will never run
the daemon (a Windows box, someone else's laptop).

## 4. Build and flash

Only the central/dongle side needs reflashing:

```bash
cd keyboards
./init.sh
# inside the container:
b rommana                                                                 # central left
zmk mabroum/rommana cd -e dongle_display -m englmaxi/zmk-dongle-display   # dongle, if used
```

`b` and `zmk` are the container's aliases for `scripts/b.sh` and
`scripts/zmk.sh`. `b rommana` builds the central left alone
(`scripts/b.sh:14`); the dongle line is what `b rommana -c` runs for it
(`b.sh:18`), without rebuilding the two peripherals too. The `.uf2` files land
in `build/artifacts/`.

## 5. Verify without the host daemon

With the keyboard connected to a Linux host:

```bash
# 1 = NORMAL, 2 = INSERT, 3 = VISUAL, 6 = RAW, 0 = off
zmk-vim-mode set normal
zmk-vim-mode set insert
zmk-vim-mode set off
```

If a dongle display is attached it shows the layer name; otherwise type a key
that differs between layers. `zmk-vim-mode devices` must list the keyboard
first; if it does not, see `scripts/spike-linux.sh`.
