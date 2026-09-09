# Integrating with the keyboards repo

Exact changes for `~/projects/keyboards`. Line references are from the state of
that repo when this was written (`4ab13206`).

## 1. Add the module

```bash
cd ~/projects/keyboards
git submodule add git@github.com:rafaelromao/zmk-vim-mode modules/rafaelromao/zmk-vim-mode
```

Then add it to the default module list in `scripts/build.sh:17`:

```diff
-DEF_MODULES=(urob/zmk-leader-key,urob/zmk-auto-layer,urob/zmk-adaptive-key,rafaelromao/zmk-layer-morph,ssbb/zmk-listeners)
+DEF_MODULES=(urob/zmk-leader-key,urob/zmk-auto-layer,urob/zmk-adaptive-key,rafaelromao/zmk-layer-morph,rafaelromao/zmk-vim-mode)
```

`ssbb/zmk-listeners` can go once the `num_lock` node below is removed; keep both
listed while you are still testing.

`CONFIG_ZMK_HID_INDICATORS=y` is already set on all eleven central/dongle
`.conf` files, and the module `select`s it anyway. Peripherals need no change.

## 2. Add the sync node to `src/features/vim.dtsi`

Two new macros, then the node. Put them inside the existing `/ { ... }` block
next to the other `VIM_MACRO` definitions:

```c
        // Host-driven visual: same shape as mc_v_vim, without typing "v".
        VIM_MACRO(vim_visual_host        , &vim_off &tog_on VIM_NORMAL &tog_on VIM_VISUAL)
```

```c
/ {
    vim_sync {
        compatible = "zmk,hid-indicator-code-listener";
        indicators = <HID_USAGE_LED_COMPOSE HID_USAGE_LED_KANA HID_USAGE_LED_SCROLL_LOCK>;
        managed-layers = <VIM_NORMAL VIM_VISUAL VIM_CHANGE VIM_LEADER VIM_INSERT VIM_REPLACE VIM_CMDLINE>;
        off-delay-ms = <60>;
        local-guard-ms = <150>;

        normal        { code = <1>; layers = <VIM_NORMAL>; };
        insert        { code = <2>; layers = <VIM_INSERT>; };
        visual        { code = <3>; layers = <VIM_NORMAL VIM_VISUAL>; };
        legacy        { code = <4>; bindings = <&vim_mode_on>; };
        cmdline       { code = <5>; layers = <VIM_CMDLINE>; };
        raw           { code = <6>; };
        legacy_silent { code = <7>; layers = <VIM_NORMAL>; };
        // code 0 is implicit: every managed layer off
    };
};
```

Notes on the choices:

- **`VIM_CHANGE` and `VIM_REPLACE` are managed but never activated by a code.**
  Both are entered through sticky behaviors (`&ntsl`, `&sl`); a layer the module
  activated would be switched off again by the next sticky release, because ZMK
  layer state is a bitmask, not a refcount. Listing them under `managed-layers`
  still lets any host code clear them, which is what you want when leaving vim.
- **Neovim's Replace maps to code 2 (INSERT), not to `VIM_REPLACE`.** On the
  keyboard `VIM_REPLACE` is the "next key is a literal" layer behind
  `r<char>`/`q<reg>`/`@<reg>`, not Neovim's `R` mode. The all-`&trans`
  `VIM_INSERT` layer with its Esc handling is the right shape for `R`.
- **Code 4 must not reuse `&vim_mode_on`.** That macro is a layer-morph whose
  "already in vim → `&none`" branch looks idempotent, but the listener clears
  every managed layer *before* invoking a code's bindings, so from code 4 the
  guard can never fire: it would always take the notify branch and send
  Esc **and** Hyper+Esc. Since the host binds Hyper+Esc to
  `zmk-vim-mode set legacy`, the keyboard would then cancel the very override
  the host had just set. Code 4 therefore selects `VIM_NORMAL` declaratively and
  binds a plain `&kp ESC` (`&vim_mode_on_host`); `&vim_mode_on` stays for the
  combos, where nothing has pre-cleared the layers and the guard does work.
  Code 7 is the same state with no binding at all, so a re-assert after a
  reconnect or a clobber never re-types Esc.
- **Code 6 (raw) lists no layers.** Keys reach the host untouched. Bind a tool
  layer here later if you want one.

## 3. Remove the num-lock listener (recommended)

`src/features/vim.dtsi:12-20` currently maps NUM LOCK to vim mode:

```c
        num_lock {
            indicator = <HID_USAGE_LED_NUM_LOCK>;
            bindings = <&vim_mode_on &vim_off>;
        };
```

Delete that node (and the surrounding `hid_listeners`/`zmk,hid-listeners` block
if nothing else uses it). Reason: on Linux, every libinput LED push carries the
kernel's real Num Lock state, so if `input:numlock_by_default` is ever true the
listener re-fires `&vim_mode_on` and injects an Esc. Manual entry into vim mode
remains available through:

- the `cb_vim_mode` combo (`src/features/combos.dtsi:36`),
- `cb_enter_vim` / `cb_leave_vim` on the MACROS layer (`combos.dtsi:221-222`),
- `zmk-vim-mode set legacy` from the host.

There is deliberately no `&to VIM_NORMAL` key on the TOGGLES layer; that layer's
`&kp KP_NUM` used to be the manual toggle, and is now just an OS num-lock key.

Keep the node only if you use these keyboards on a machine that will never run
the daemon (a Windows box, someone else's laptop).

## 4. Build and flash

Only the central/dongle side needs reflashing:

```bash
cd ~/projects/keyboards
./init.sh
# inside the container:
b rommana cl      # central left
b rommana cd      # dongle, if used
```

## 5. Verify without the host daemon

With the keyboard connected to the Omarchy box:

```bash
# 1 = NORMAL, 2 = INSERT, 3 = VISUAL, 6 = RAW, 0 = off
zmk-vim-mode set normal
zmk-vim-mode set insert
zmk-vim-mode set off
```

If a dongle display is attached it shows the layer name; otherwise type a key
that differs between layers. `zmk-vim-mode devices` must list the keyboard
first; if it does not, see `scripts/spike-linux.sh`.
