<!-- Implementation plan produced 2026-09-03 in a planning session. Source of truth for the implementation session; update as decisions change. -->

# zmk-vim-mode — sync ZMK keyboard layers with the editor's vim state (Omarchy first, macOS next)

## Context

The user's ZMK keyboards (`/Users/rromao/projects/keyboards`, one shared keymap for 8 boards) have a "VIM Mode":
seven layers `VIM_NORMAL`=3 `VIM_VISUAL`=4 `VIM_CHANGE`=5 `VIM_LEADER`=6 `VIM_INSERT`=7 `VIM_REPLACE`=8
`VIM_CMDLINE`=9 (`src/definitions/config.dtsi:10-45`). The host tells the keyboard **one bit** (NUM LOCK LED) via
`ssbb/zmk-listeners` (`src/features/vim.dtsi:1-20`: on → `&vim_mode_on`, off → `&vim_off`); everything else
(normal/insert/visual/…) is *guessed on the keyboard* by intercepting `i a o s c r v V ^V / : ! Esc Enter x d y p
= ~ J q @` (`vim.dtsi:75-154`). `vim_normal_layer` (`keymap.dtsi:46-54`) remaps physical keys to vim letters and
has `&none` on ~14 positions, so a wrong NORMAL state *drops keystrokes*.

Host side today = two focus-heuristic scripts (installed copies in `~/dotfiles`):

| OS | Detect | Signal |
|---|---|---|
| macOS | Hammerspoon `~/.hammerspoon/zmk-vim-mode-watcher.lua`: app == Code/Obsidian, or Ghostty title contains `nvim` | `setleds +num/-num` (setledsmac, IOKit) |
| Omarchy/Hyprland | systemd user unit → `socat` on Hyprland `.socket2.sock` `activewindow>>class,title` | `hyprctl keyword input:numlock_by_default` ("only takes effect after next keypress", one-way) |

Problems: mode inferred, not known (plugin mappings, `:startinsert`, pickers, LSP actions desync); one-bit edge
channel injects ESC on duplicate edges (fixed twice: `b3dc1922`, `d63277f3`); title sniffing, no tmux; Linux
path mutates real Num Lock; no re-assert after BLE reconnect; allow-lists duplicated; ~20 manual install steps.

**Explicit requirement (user):** the keyboard must also drop out of vim layers whenever the editor is in a
context that expects raw keys or free text — after a `<leader>` sequence lands in a picker/prompt, in plugin
UI buffers whose letters are commands (dashboard, explorer, lazy/mason, trouble), in terminal mode, and when a
**tool window without vim mode** is focused inside a vim-enabled editor (VSCode terminal/search/palette,
IntelliJ tool windows, Obsidian settings/search). Today's watchers cannot see any of this.

Hardware/env: keyboard **"Rommana"** VID `0x1d50` PID `0x615e`, BLE (`MaxOutputReportSize=2`), central/dongle
confs have `CONFIG_ZMK_HID_INDICATORS=y`. Linux box = Omarchy (Arch + Hyprland, Ghostty, LazyVim). Mac = macOS
26.6, Neovim 0.11.5 (LazyVim, `vim.uv` present), Ghostty, tmux (no `focus-events`), Hammerspoon, Go 1.26.
The Mac is where the keyboard is connected *right now*; Omarchy is the v1 target (user decision) — the keyboard
switches hosts via BLE profiles.

## Decisions (confirmed with user)
1. Daemon in **Go** (single codebase; Linux pure Go; macOS later via small cgo shims). Only dependency: `golang.org/x/sys`.
2. **Monorepo** at `/Users/rromao/projects/zmk-vim-mode`: daemon + Neovim plugin + ZMK module + editor plugins.
3. **Keep LEGACY** (focus-based, keyboard-inferred) mode for apps without an integration.
4. **Omarchy first for everything** (daemon, tool-window awareness, install); macOS backend is a later phase.
   Linux focus backend v1 = Hyprland only (interface open for X11/GNOME/KDE).
5. **RAW code = no vim layers** (plain base layout); distinct code so a tool layer can be bound later.
6. Editor integrations (researched, §4): **VSCode → switch to `vscode-neovim`** (real modes via our Neovim plugin,
   tiny companion extension for focus, TTL-based `raw`); **Obsidian → own community plugin**; **IntelliJ →
   deferred** (not installed; IdeaVim extension plugin when needed). AT-SPI2 rejected as primary mechanism.

## Approach

### Transport: HID LED indicator bits — 3 bits, 8 codes
Reuse the channel that already works over USB and BLE with zero pairing/protocol work: the keyboard output
report. ZMK declares 5 LED usages (Num 0x01, Caps 0x02, Scroll 0x04, Compose 0x08, Kana 0x10 as
`HID_INDICATOR_*`; `app/include/zmk/hid.h` descriptor: 5 bits + 3 CONST pad). Use the three bits no OS drives
as a *lock*: **b0 = Compose, b1 = Kana, b2 = Scroll Lock**. Never touch Caps; Num Lock left to the user
(see firmware §1 on the old listener).

| code | meaning | keyboard action |
|---|---|---|
| 0 | OFF — no vim editor focused | all managed layers off |
| 1 | NORMAL (incl. operator-pending, `nt`, `ni*`) | `VIM_NORMAL` |
| 2 | INSERT (also Replace `R*` — see §1) | `VIM_INSERT` |
| 3 | VISUAL / select | `VIM_NORMAL` + `VIM_VISUAL` (same shape as `mc_v_vim`) |
| 4 | LEGACY transition — vim-like app with no mode feed; keyboard infers like today | `&vim_mode_on` binding (one ESC) |
| 5 | CMDLINE (`c*`, hit-enter/more/confirm prompts) | `VIM_CMDLINE` |
| 6 | RAW — keys must pass through: plugin UI buffers, terminal mode `t`, pending `<leader>`, non-vim tool window | no vim layers (distinct from OFF) |
| 7 | LEGACY_SILENT — re-assert of 4 after a clobber/reconnect | `VIM_NORMAL`, no bindings (no replayed ESC) |

RAW ≠ INSERT on purpose: INSERT keeps the keyboard's local inference alive (Esc → `vim_esc` → NORMAL), which
is wrong in a terminal job or a tool window where Esc belongs to the app. RAW has no vim layers → no inference.
ZMK stores the indicator byte **per endpoint/profile** and re-raises the event on profile switch, so a keyboard
shared by the Mac and the Linux box keeps each host's last code.

### Linux LED path: hidraw writes + evdev echo (verified in libinput/Hyprland/kernel source)
libinput ≥ 1.26 writes **all five** LEDs whenever a compositor pushes LED state; Hyprland pushes on **every key
and modifier event to every keyboard, uncached** (`CInputManager::updateKeyboardsLeds`); wlroots/Aquamarine
also zero all LEDs on device add. Compositors never *set* Compose/Kana (no xkb indicator map), they only clear
them. So:
- **Write through `/dev/hidrawN`** (`[0x01, leds]`): the kernel's own LED cache (`dev->led`) stays 0 for our bits,
  so Hyprland's per-keystroke "write 0" is dropped by `input_get_disposition` (no change → no HID report).
  Kernel path verified: hidraw → `hid_hw_output_report` → USB `SET_REPORT` / `UHID_OUTPUT` → BlueZ `forward_report`
  (strips report id) → GATT write.
- **Read `EV_LED` echoes on the paired `/dev/input/eventN`** (kernel broadcasts LED changes to all evdev
  readers): the kernel emits its own report only when a bit *it* tracks changes (user toggles Caps/Num,
  `numlock_by_default`), which clobbers our bits → immediate hidraw re-write carrying the kernel's current
  Num/Caps (from `EVIOCGLED`). Apply `EVIOCSMASK` with an empty `EV_KEY` mask so the daemon never receives
  keystrokes (privacy/LGPD, CPU).
- Firmware belt-and-braces: code 0 is applied only after `off-delay-ms` (60) of stability, so a clobber +
  rewrite pair never drops the layer; re-asserts of LEGACY use code 7 so no ESC is replayed.
- X11 bonus (later): `xset led 4` drives `LED_COMPOSE` through XKB indicator 4, no udev needed.
- macOS (later): `IOHIDDeviceSetValue` on `kHIDPage_LEDs` elements; the OS never rewrites these LEDs, only
  device add/wake need re-assert.

### Hybrid, not replacement — with a guard
Keep the keyboard's local inference (zero-latency feel; needed for `CHANGE` after `c` and `r<char>`/`q<reg>`
literals, which Neovim cannot report). Host codes are *corrections* that converge the keyboard within one BLE
round trip. Safety rules:
- Host sends only coarse states; operator-pending `no*` is not sent (keeps the CHANGE sticky alive); daemon
  coalesces bursts (latest wins, 10 ms) and never writes an unchanged code.
- **Firmware local-transition guard**: a keyboard-caused change to a managed layer (`zmk_layer_state_changed`
  not raised by the module itself) parks incoming host codes for `local-guard-ms` (150); the latest parked code
  is applied when the guard expires. Keyboard authoritative in motion, host authoritative at rest. This closes
  the "stale host code lands after a faster local transition" race (`<CR>i`, `Esc`→`c` rolls) that a 10 ms
  daemon debounce cannot.
- Host codes never *activate* `VIM_CHANGE` or `VIM_REPLACE` (sticky `&mo`/`&ntsl` layers; a module-activated
  copy would be killed by the sticky release since layer state is a bitmask, not a refcount). They stay in
  `managed-layers` so OFF/RAW/other codes clear them. Neovim `R*` therefore maps to INSERT (all-`&trans`
  layer with Esc handling — the right shape for Replace mode).
- `<leader>` sequences become host-driven (RAW while pending, real mode when resolved), fixing today's
  "stuck in LEADER after `<space>ff`" drift.

### Alternatives rejected (researched on zmkfirmware/zmk main + zmk-studio-messages)
- **ZMK Studio RPC**: no `set_active_layer`/layer notifications; all keymap handlers require physical unlock;
  RPC only on the selected endpoint; Studio use disables `.keymap` edits until "Restore Stock Settings".
- **Raw HID** (`zzeneg/zmk-raw-hid`, usage page 0xFF60; hosts `rawtalk`, `qmk-hid-host`): clobber-free and the
  fallback if LED bits ever prove unworkable, but BLE needs a second HIDS GATT service (unverified on macOS).
  The daemon's LED backend is an interface, so a later swap touches one package + one firmware listener.
- CDC serial (USB only), uinput Num Lock (1 bit, real lock state), kanata/keyd (doesn't change what ZMK sends).

## Components (monorepo)

```
zmk-vim-mode/
├── cmd/zmk-vim-mode/                daemon|set|status|devices|install|uninstall|doctor|version
├── internal/{proto,state,server,focus,leds,platform,install,doctor}
│   ├── focus/{hyprland,noop}  (+ darwin later)      leds/{linux}  (+ darwin later)
├── lua/zmk-vim-mode/{init.lua,modes.lua}, plugin/zmk-vim-mode.lua, tests/lua/   Neovim plugin (repo root = lazy.nvim plugin)
├── zephyr/module.yml, firmware/{Kconfig,CMakeLists.txt,dts/bindings/,src/}   ZMK module (repo root = Zephyr module)
├── editors/{vscode,intellij,obsidian}/               editor clients speaking the same socket protocol (§4)
├── contrib/{systemd,udev,launchd,nvim-lazy-spec.lua,hyprland-bind.conf}
├── Makefile, README.md
```
Consumed as: `go install`, lazy.nvim `{"rafaelromao/zmk-vim-mode"}`, and `modules/rafaelromao/zmk-vim-mode`
submodule in the keyboards repo (`scripts/build.sh:17 DEF_MODULES`, `-DZMK_EXTRA_MODULES` path convention
`modules/<owner>/<repo>` requires `zephyr/module.yml` at repo root — `build: cmake: firmware`, `kconfig:
firmware/Kconfig`, `settings: dts_root: firmware`).

### 1. Firmware module — `zmk,hid-indicator-code-listener` (generic, reusable)
Modelled on `ssbb/zmk-listeners` `main` (`src/hid_listeners.c`: `ZMK_LISTENER`/`ZMK_SUBSCRIPTION`, `LISTIFY(...
ZMK_KEYMAP_EXTRACT_BINDING ...)`, `zmk_behavior_queue_add` with `.position = INT32_MAX`, `.source =
ZMK_POSITION_STATE_CHANGE_SOURCE_LOCAL`), but decoding a multi-bit code and driving layers directly with
`zmk_keymap_layer_activate/deactivate(id, false)` (instant; `vim_off` as a macro is 7 toggles ≈ 70 ms).

ZMK facts that shape it (`app/src/hid_indicators.c`, `behavior_queue.c`): the event fires on **every** LED
report (no change check) and on **endpoint switch**, coalescing to the latest value via a single `K_WORK` ⇒
protocol is *state*, module acts only when the decoded code differs from `last_received`. Layer numbers passed
to `&tog_on VIM_NORMAL` are layer **IDs**, so DT values are used as IDs directly (no `index_to_id`).

Keymap node (in `src/features/vim.dtsi`):
```dts
#include <dt-bindings/zmk/hid_usage.h>
/ {
    vim_sync {
        compatible = "zmk,hid-indicator-code-listener";
        indicators = <HID_USAGE_LED_COMPOSE HID_USAGE_LED_KANA HID_USAGE_LED_SCROLL_LOCK>;  // b0 b1 b2
        managed-layers = <VIM_NORMAL VIM_VISUAL VIM_CHANGE VIM_INSERT VIM_REPLACE VIM_CMDLINE>;  // VIM_LEADER removed, see phase 3 notes
        off-delay-ms = <60>; local-guard-ms = <150>;
        normal        { code = <1>; layers = <VIM_NORMAL>; };
        insert        { code = <2>; layers = <VIM_INSERT>; };
        visual        { code = <3>; layers = <VIM_NORMAL VIM_VISUAL>; };
        legacy        { code = <4>; layers = <VIM_NORMAL>; bindings = <&vim_mode_on_host>; };  // plain Esc; lives in vim_legacy.dtsi (phase 3 notes)
        cmdline       { code = <5>; layers = <VIM_CMDLINE>; };
        raw           { code = <6>; };                                // no vim layers
        legacy_silent { code = <7>; layers = <VIM_NORMAL>; };
        // code 0 implicit: all managed layers off
    };
};
```
Semantics: `decode(indicators)` → if ≠ `last_received`: `schedule(code)`. `schedule` uses one
`k_work_delayable`: delay = remaining local guard (if a non-self managed-layer change happened < 150 ms ago),
and ≥ 60 ms when code == 0; a code equal to `last_applied` cancels a pending OFF (a clobber that never landed)
without re-running bindings. `apply(code)`: with `applying=true`, deactivate managed layers not wanted,
activate wanted ones; then queue `bindings` press/release (check `< 0` return, log). `SYS_INIT` seeds
`last_received/last_applied` from `zmk_hid_indicators_get_current_profile()`. Everything runs on the system
workqueue (Rommana confs already set `CONFIG_SYSTEM_WORKQUEUE_STACK_SIZE=4096`).

Plumbing: `firmware/Kconfig` — `DT_COMPAT_… := zmk,hid-indicator-code-listener`; `config
ZMK_HID_INDICATOR_CODE_LISTENER` `bool`, `default $(dt_compat_enabled,…)`, `depends on !ZMK_SPLIT ||
ZMK_SPLIT_ROLE_CENTRAL`, `select ZMK_HID_INDICATORS` (zmk-listeners forgets this). `firmware/CMakeLists.txt` —
`target_sources_ifdef(CONFIG_ZMK_HID_INDICATOR_CODE_LISTENER app PRIVATE src/hid_indicator_code_listener.c)`.
Binding YAML: `indicators: array req`, `managed-layers: array req`, `off-delay-ms`/`local-guard-ms`/`tap-ms`/
`wait-ms: int`; child `code: int req`, `layers: array`, `bindings: phandle-array`. Use
`DT_INST_FOREACH_CHILD_STATUS_OKAY`, `static` helpers, explicit `#include <zmk/events/position_state_changed.h>`
and `<zmk/events/layer_state_changed.h>`.

Keyboards repo changes: add submodule `modules/rafaelromao/zmk-vim-mode` + `DEF_MODULES` entry
(`scripts/build.sh:17`); add the `vim_sync` node to `vim.dtsi`. **Recommend removing** the old `num_lock`
`zmk,hid-listeners` node (`vim.dtsi:12-20`) and `ssbb/zmk-listeners` from `DEF_MODULES`: nothing else uses the
module, and if `input:numlock_by_default` is ever left `true` every libinput LED push would re-fire
`&vim_mode_on` (ESC). Manual toggles remain: combo `cb_vim_mode` (`combos.dtsi:36`), `&to VIM_NORMAL`
(TOGGLES), and `zmk-vim-mode set legacy`. No new macros needed (`layers =` supersedes). No peripheral changes.
Rebuild + flash centrals/dongles (`./init.sh` → `b <kbd>`) — the one unavoidable manual step.

### 2. Host daemon `zmk-vim-mode` (Go) — Linux first
- **Inputs.** (a) Unix socket, fixed path `$HOME/.local/state/zmk-vim-mode/daemon.sock` (ignore XDG on both
  ends so launchd/systemd/tmux agree; dir 0700, socket 0600 via `umask`, `flock` on `daemon.lock`, dial-before-
  unlink for stale sockets, optional peer-UID check). Newline-JSON from *clients* — Neovim now, editor plugins
  later (§4) — see Protocol. Connection close = client gone. (b) Focus: Hyprland — never require
  `HYPRLAND_INSTANCE_SIGNATURE` (the old script's "restart the computer" root cause); glob
  `$XDG_RUNTIME_DIR/hypr/*/.socket2.sock` (fallback `/run/user/$(id -u)`), pick the instance whose
  `.socket.sock` answers `j/version`, inotify `hypr/` to survive Hyprland restarts, query `j/activewindow` on
  connect, parse `activewindowv2>>addr` + `j/clients` (titles contain commas). Move `org.omarchy.nvim` from the
  legacy list to the terminal list. Interface `focus.Watcher` for later X11/wlr-toplevel/GNOME/macOS backends.
  (c) Devices: inotify on `/dev/input` + `/dev/hidraw*` for `IN_CREATE|IN_ATTRIB|IN_DELETE` (udev ACL arrives
  as `IN_ATTRIB` after create — retry on `EACCES/ENOENT`); probe with `EVIOCGID` (bustype 0x03 USB / 0x05 BLE,
  VID/PID), `EVIOCGNAME`, `EVIOCGBIT(0)` has `EV_KEY&EV_LED`, `EVIOCGBIT(EV_LED)` has `LED_COMPOSE&LED_KANA`
  (default device filter — Apple/non-ZMK boards lack them; if absent, firmware indicators are off →
  `doctor` says so); pair eventN ↔ hidrawN via the common `hid` sysfs parent. `x/sys/unix` lacks
  `EVIOC*`/`LED_*`/`input_event`: define locally (`_IOC(dir,'E',nr,size)`, `input_event` 24 bytes on 64-bit).
  (d) Context classifier for non-Neovim apps (§4). (e) Manual override `set`.
- **Decision** (pure `Decide(rules, snapshot)`, table-tested), priority: active override → frontmost app has a
  connected editor client → `legacy_apps` (with AT-SPI context if available: vim → 4, raw → 6; else 4) →
  `terminal_apps ∪ gui_nvim_apps` → highest-ranked Neovim client's mode → title heuristic (`nvim` in title → 4,
  Hyprland only) → 0. Client ranking: tri-state `focused` (unknown/true/false; unknown ranks above explicit
  false so a no-focus-events setup degrades to "most recently active wins"), then latest focus seq, `nested`,
  latest event seq. Frontmost *unknown* (focus backend down) fails **open** (trust clients), never OFF.
- **Reconciler.** `SetDesired(code)` coalesces 10 ms, writes to all matching devices, dedupes per device;
  `Reassert(dev)` writes `SilentAlias(desired)` (4 → 7) on device add (retries +50/+300/+1000 ms), on `EV_LED`
  echo differing from desired (debounced ≥ 10 ms, ≤ 20/s), on wake. Daemon start: apply positive codes at
  once, wait ~1.5 s before the first OFF (lets clients reconnect); on SIGTERM write 0. Writer goroutine with a
  200 ms watchdog log. Pulses through 0 (if ever used) must be ≥ 25 ms apart (two BLE connection events).
- **CLI**: `daemon`, `set <off|normal|insert|visual|cmdline|raw|legacy|auto> [--ttl 30s] [--sticky]` (override
  replaces the automatic decision; expires on `set auto`, TTL, frontmost identity change unless `--sticky`,
  or daemon restart; re-issuing the same `set` toggles back to `auto` so one hotkey works), `status`
  (frontmost, clients, override, devices, last codes), `devices`, `install`, `uninstall`, `doctor`.
  No config file in v1: defaults + flags (`--legacy-app`, `--terminal-app`, `--socket`, `--log-level`). Logging
  via `log/slog` to stderr (journald/launchd capture); window titles only at debug; log files 0600.

### 3. Neovim plugin (`lua/zmk-vim-mode/`, `plugin/zmk-vim-mode.lua`)
- Transport: `vim.uv.new_pipe():connect(sock)` persistent; JSON lines; reconnect backoff 250 ms → 2 s; `hello`
  (with current mode + focus) on **every** connect; silent no-op when the daemon is absent. Only `api-fast`
  calls (`nvim_get_mode`) inside uv callbacks, everything else via `vim.schedule`. Skip when headless
  (`#nvim_list_uis()==0`, re-check on `UIEnter`). `hello` carries `nested = vim.env.NVIM ~= nil`, `tmux`,
  `vscode = vim.g.vscode ~= nil`. Warn once if `$TMUX` is set and `tmux show -gv focus-events` ≠ `on`.
- **Effective state** = f(mode, buffer/window context, leader-pending), recomputed on `ModeChanged *:*`,
  `BufEnter`, `WinEnter`, `FileType`, `TermEnter/TermLeave`, `SafeState` (deduped; only changes are sent), plus
  `FocusGained/FocusLost`, `VimSuspend/VimResume`, `VimLeavePre` (`bye`; `VimLeave` is skipped when `v:dying ≥ 2`,
  so the daemon relies on socket close). `ModeChanged` beats `InsertLeave` (misses `i_CTRL-C`) and
  `CmdlineEnter` (fires inside mappings).
- Rules, in order: (1) `leader_pending` → `raw` — set by `vim.on_key` when `typed == mapleader/maplocalleader`
  in normal/visual; cleared on the next `SafeState` (does not fire while a mapping is pending), on
  `ModeChanged`, or by a `timeoutlen + 50 ms` timer. Covers which-key menus. (2) mode `t` → `raw`. (3) raw
  buffer → `raw`: `filetype ∈ raw_filetypes` (default `snacks_dashboard dashboard alpha lazy mason neo-tree
  NvimTree oil Trouble trouble snacks_picker_list snacks_explorer TelescopeResults fugitive DiffviewFiles dap-repl
  dapui_* Outline aerial noice notify lspinfo`), or `buftype nofile/prompt` + `nomodifiable` and not in
  `raw_exceptions` (`help qf man checkhealth`), or a floating window (`nvim_win_get_config(0).relative ~= ""`)
  with a non-empty filetype not in exceptions. Insert-mode prompts (pickers, `vim.ui.input`) stay `insert`.
  `opts.classify(buf, win, mode) -> state|nil` overrides everything. (4) coarse map (`:h mode()` 0.11.5,
  leading chars): `n ni* nt ntT → normal`; `no* → skip`; `v V ^V vs Vs ^Vs s S ^S → visual`; `i ic ix → insert`;
  `R Rc Rx Rv Rvc Rvx → insert`; `c cr cv cvr r rm r? → cmdline`; `! → skip`.
- Commands `:ZmkVimMode status|reconnect`. Opts: `socket, raw_filetypes, raw_exceptions, terminal_state,
  leader_raw, classify, debug`. `setup()` is called by lazy.nvim `opts = {}` (never auto-run from `plugin/`).
  User spec `lua/plugins/zmk-vim-mode.lua`: `{ "rafaelromao/zmk-vim-mode", lazy = false, opts = {} }` (or
  `dir = "~/projects/zmk-vim-mode"` while developing). Tests: `nvim --headless -l tests/lua/modes_spec.lua`.

### 4. Non-Neovim editors: VSCode, Obsidian, IntelliJ
Principle: each editor gets a small **client** speaking the same socket protocol (`hello/mode/focus`),
reporting the real vim mode *and* whether focus sits in the vim-enabled editor component (→ mode) or in a tool
window/prompt (→ `raw`). An app with a connected client outranks the `legacy_apps` rule.

What is installed here (verified locally):

| Editor | Present | Vim state |
|---|---|---|
| VSCode | `/Applications/Visual Studio Code.app` | `vscodevim.vim` 1.32.4, `vim.vimrc.enable: true`, `vim.vimrc.path: ~/.vscodevimrc` (empty); an `extensions.experimental.affinity` entry for `asvetliakov.vscode-neovim` exists but that extension is **not** installed. Linux VSCode settings have no vim config. |
| Obsidian | `/Applications/Obsidian.app`, vault `~/projects/secondbrain` | `vimMode: true`; `obsidian-vimrc-support` 0.10.2 and `edit-in-neovim` 1.4.0 present but **disabled** |
| IntelliJ | not installed (no app, no `~/.ideavimrc`, no JetBrains config) | defer IdeaVim until it is actually used |

**AT-SPI2 is rejected as the primary mechanism** (research killed the earlier fallback idea): it cannot see vim
mode at all; Chromium maps Monaco's hidden textarea, the quick-input box, the find widget and xterm.js's helper
textarea all to the same `ATK_ROLE_ENTRY`, so role alone cannot separate editor from tool window; enabling
accessibility makes VSCode switch Monaco to a slower, less virtualised rendering strategy; and JetBrains has no
Linux screen-reader support before 2026.2. Keep it only as a last-resort refinement. Free and worth taking
instead: Hyprland's own `activewindow` event already gives app-level focus (alt-tab away → OFF).

#### 4a. VSCode → switch to `asvetliakov.vscode-neovim`
Numbers (Marketplace/GitHub, Sep 2026): VSCodeVim 9.0M installs but **1,799 open issues** (a TypeScript
re-implementation of Vim with a permanent fidelity backlog); vscode-neovim 665k installs, **66 open issues**,
released 2026-05-12, embeds real Neovim. They are mutually exclusive (both claim the `type` command). For a
heavy Neovim user the embedded-nvim option is the consensus, and the only one that can report exact modes.
- **Mode: no new code.** The embedded nvim runs our Neovim plugin (`vim.g.vscode` set, `vim.uv` available), so
  the existing socket writer works unchanged. Neither extension exposes an API: VSCodeVim's `activate` returns
  `void` and its `vim.mode` is a `when`-clause context key that extensions cannot read (`setContext` has no
  getter — vscode#10471 open since 2016); vscode-neovim likewise only sets `neovim.mode`.
- **Focus is unobservable from the extension API.** `vscode.d.ts` and all 40 proposed-API files have nothing for
  "which part has keyboard focus"; `onDidChangeActiveTextEditor` does not fire when focus moves to the terminal
  or quick-input; `activeTerminal` means "focused *or most recently* focused"; `TerminalState.isInteractedWith`
  latches. The first idea — a short TTL on nvim's mode traffic, lapsing to `raw` — was wrong: idle in NORMAL
  (reading code) is the common case, and `raw` there sends base-layout letters as vim commands. Implemented
  instead (phase 4):
  1. **Window title.** `"window.title": "${dirty}${activeEditorShort}${separator}${rootName} [${focusedView}]"`
     publishes the focused view (`terminal`, `Explorer`, `Search`, …; **`Text Editor`** while the text editor
     has focus — verified on the box; the plan's "empty" assumption held only for older builds). The daemon
     follows Hyprland `windowtitle(v2)` events for the focused window (re-querying `j/activewindow`, the
     source of truth); `AppRule.TitleView` (`\[([^\[\]]*)\]\s*$`) extracts the marker and any value outside
     `AppRule.EditorViews` (`""`, `Text Editor`, `Editor`) is `raw`, above every client. Zero VSCode code.
  2. **Companion extension** (`editors/vscode/`, plain JS, no build step): the app's *own* client
     (`client: vscode, app: vscode`). Says `raw` while `activeTextEditor` is undefined (Settings, webviews,
     images, empty window) and, with a TTL, after wrapping Ctrl+P / Ctrl+Shift+P / F1 / Ctrl+G / Ctrl+Shift+O
     (`when: editorTextFocus`); says `none` — new wire mode, "no opinion" — otherwise, so the embedded nvim
     decides. The hint clears on active-editor, selection or window-state change, else it expires (20 s).
     Known glitch: Escape out of the palette, then the first motion key still goes through the base layout.
  3. **Neovim-side hint** (found on the box: a vim user opens the palette from mappings, never from
     Ctrl+Shift+P, so the companion's keybinding wrappers never fired). The plugin wraps `vim.rpcnotify`/
     `rpcrequest` and `require("vscode").action/call`; a command matching `vscode_raw_actions` (quick inputs,
     rename — not views/terminal, which the title handles both ways) makes the nvim client report `raw` until
     a typed key reaches Neovim again, the companion reports the close (`zmkVimMode.rawHint` forward,
     `vscode-neovim.lua` callback), or a 20 s TTL.
  4. **Accessibility bus (opt-in, `--atspi`)** — the user chose it after seeing the mouse-opened Command
     Center stay in NORMAL. `internal/dbus` is a minimal wire client (no dependency: AT-SPI apps only emit to a
     listener registered from a *live* connection, so shelling out to `busctl` cannot work, and no Go binding
     was available offline). `internal/focus/atspi` sets `org.a11y.Status.IsEnabled`, registers
     `object:state-changed:focused`, joins events to windows by pid (Electron's a11y connection is the main
     process, the same pid Hyprland reports), reads role/name/`tag`/`class`/`roledescription` and classifies:
     Monaco text area → editor unless a known widget name/ancestor (find, replace, SCM, settings, rename…);
     everything else → other. `Decide`: title → widget other → Raw; widget editor → clients' raw hints ignored.
     The daemon sends `editor_focus` to the app's clients on transitions so hints clear with no first-key
     glitch. `atspi-watch` prints events for tuning. Cost: accessibility on for the whole session (what a
     screen reader does); `install --vscode` sets `editor.accessibilitySupport: off` to keep Monaco normal.
     Verified on the box (2026-09-10), three corrections along the way: the registry wants the event in its
     D-Bus spelling (`Object:StateChanged:Focused`, what libatspi sends) or bridges ignore the listener;
     both `IsEnabled` and `ScreenReaderEnabled` must be set; and Electron still exposes nothing of its DOM
     without `--force-renderer-accessibility`, which `install --vscode` now writes to
     `~/.config/code-flags.conf`. Modern Monaco's input is a `native-edit-context` div (roledescription
     "editor"), the palette a `monaco-list-row` list item.
  5. **Decide** combines them per app: title → a11y widget → own client `raw` → best opinionated client →
     `legacy`.
- Zero migration cost: `~/.vscodevimrc` is empty. Uninstall VSCodeVim (the affinity setting already present is
  vscode-neovim's documented setup). Cheap interim if the switch is postponed: VSCodeVim's built-in
  `vim.autoSwitchInputMethod.switchIMCmd` shells out on insert-like transitions — binary only, no focus info.

#### 4b. Obsidian → community plugin in `editors/obsidian/`
- Mode: `cm.on('vim-mode-change', cb)` → `{mode, subMode}`, emitting exactly `normal | insert | visual | replace`.
  **There is no `cmdline` event** — `:` and `/` go through `openDialog`, whose input takes DOM focus — so
  `cmdline` is a `focusin` landing inside the editor container but outside `.cm-content` (`.cm-vim-panel`).
  Implemented that way in phase 4 (`editors/obsidian/`, plain JS, no build step); `vim-mode-change` only
  triggers a re-read, the mode itself comes from `cm.state.vim` (`insertMode`, `visualMode`) with the event's
  `mode` preferred when it belongs to the active adapter. Editor handle: `(view as any).editMode?.editor?.cm?.cm` with a fallback
  to the older `sourceMode?.cmEditor?.cm?.cm`; adapter at `window.CodeMirrorAdapter`.
- Re-register listeners on **both** `active-leaf-change` and `file-open` (needed when the same file opens in a
  new pane) — the pattern `obsidian-vimrc-support` uses, already on disk to read.
- `raw`: one capturing `focusin` listener per window; any target not inside `.cm-content` → `raw` (one stable
  CodeMirror class instead of six fragile Obsidian ones; fails safe). Also `getMode() === 'preview'` → `raw`;
  window blur/focus via `registerDomEvent` (popouts via `window-open`).
- Transport: `isDesktopOnly: true`, lazy `require('net')` behind `Platform.isDesktopApp`, use `this.app`.
  Existing Obsidian vim plugins all shell out to an IM-switcher binary; none speaks a socket.

#### 4c. IntelliJ → defer; when needed, an IdeaVim extension plugin
`<depends>IdeaVIM</depends>` + the `IdeaVIM.vimExtension` extension point (as IdeaVim-EasyMotion and
idea-which-key do). The new thin API `com.intellij.vim.api` has exactly the three callbacks needed —
`listeners { onModeChange {}; onEditorFocusGain {}; onEditorFocusLost {} }` — but is marked **EXPERIMENTAL** and
its `api` module has no `maven-publish`, so verify out-of-tree consumability first (the loader supports it:
`IjPluginExtensionsScanner` reads `META-INF/extensions.json` from any plugin depending on IdeaVim). Fallback:
the `@Internal` `injector.listenersNotifier.modeChangeListeners` plus per-editor
`EditorEx.addFocusListener(FocusChangeListener)`. `raw` = focus lost **and**
`FileEditorManager.getInstance(project).selectedTextEditor == null` (the idiom IdeaVim's own mode widget uses).
Terminals, consoles and commit-message fields **are** IdeaVim editors, force-switched to INSERT — already the
right pass-through, do not map them to `raw`. Transport: Java 16+ `UnixDomainSocketAddress` + `SocketChannel`
(JetBrains Runtime is JBR 17/21), off the EDT, reconnect on EPIPE.

#### 4d. Cross-cutting protocol notes
- `raw` is the **fail-safe default**: assert it on any uncertainty so an unrecognised tool window never traps
  the user in a non-passthrough layer.
- Tolerate a **missing `cmdline`** (Obsidian) and **richer modes than we model** (IdeaVim's 20-value enum incl.
  `OP_PENDING*`, `SELECT_*`): map unknown → `normal`, log once.
- Daemon → client **`resync`** request; every client seeds state on connect (all three APIs can read current
  state synchronously: `vscode.eval`, `VimApi`, `editor.state.vim`). Needed after daemon restarts.
- `ttl_ms` on `mode`/`hello`: the report expires after it, leaving the client with **no opinion** (wire mode
  `none`, also sendable explicitly). Used by the VSCode companion's quick-input hint; never to downgrade a mode
  to `raw` (see 4a for why).

### 5. Install
- Linux: `make install` → build (`CGO_ENABLED=0`), copy to `~/.local/bin`, `~/.config/systemd/user/
  zmk-vim-mode.service` (`After=/PartOf=graphical-session.target`, `Restart=always`, `RestartSec=2`), udev
  `/etc/udev/rules.d/60-zmk-vim-mode.rules` (must sort before 73-seat-late; the only `sudo`):
  ```
  SUBSYSTEM=="hidraw", KERNELS=="0005:1D50:615E.*", TAG+="uaccess"                              # BLE
  SUBSYSTEM=="hidraw", ATTRS{idVendor}=="1d50", ATTRS{idProduct}=="615e", TAG+="uaccess"         # USB
  SUBSYSTEM=="input", KERNEL=="event*", ATTRS{id/vendor}=="1d50", ATTRS{id/product}=="615e", TAG+="uaccess"
  ```
  (`uaccess` ACLs need an active logind seat session — fine for a graphical-session unit; installer offers
  `GROUP="input", MODE="0660"` as the deterministic fallback.) `install --nvim/--tmux` *print* the lazy spec and
  `set -g focus-events on` (tmux default off; Ghostty implements DEC 1004 — verified) instead of editing
  dotfiles. Retire old pieces: `hyprland.conf:24-25` exec-once lines, `systemctl --user disable
  zmk-vim-mode-watcher`, `~/.hammerspoon/init.lua` `require`.
- macOS (later phase): launchd agent `~/Library/LaunchAgents/dev.rafaelromao.zmk-vim-mode.plist` with
  `LimitLoadToSessionType=Aqua`, `ProcessType=Interactive`, `KeepAlive`, `RunAtLoad`, absolute
  `StandardErrorPath`, loaded via `launchctl bootstrap gui/$UID`. TCC for `IOHIDDeviceSetValue` is unverified
  and must be spiked *from launchd* (today's `setleds` runs as Hammerspoon's child, so it proves nothing);
  `doctor` detects `kIOReturnNotPermitted` and opens the Privacy pane.
- `doctor` v1: daemon reachable; devices found (+LED caps); LED write result; old watchers still installed;
  tmux focus-events; (Linux) udev ACL present, `hyprctl getoption input:numlock_by_default` is false.

## Protocol (v1, newline JSON over the unix socket)
Client → daemon: `{"v":1,"t":"hello","client":"nvim|vscode|intellij|obsidian","pid":N,"mode":"normal",
"focused":true,"nested":false,"tmux":true,"plugin":"0.1.0"}` · `{"t":"mode","mode":"insert"}` ·
`{"t":"focus","focused":false,"mode":"insert"}` · `{"t":"bye"}`. `mode ∈ off|normal|insert|visual|cmdline|raw`
(clients never send `legacy`); `focused` absent = unknown; every message may carry `mode` so a focus switch is
right instantly; optional `ttl_ms` on `mode` (VSCode client) downgrades the client to `raw` when it lapses.
Daemon → client: `{"v":1,"t":"welcome","daemon":"0.1.0","code":1}`, `{"t":"resync"}` (client re-sends current
mode + focus), or `{"t":"error","code":"unsupported_version"}` + close. CLI: `{"v":1,"t":"set","mode":"legacy","ttl_ms":0,"clear_on_focus_change":true}`,
`status`, `devices` → one response line. Unknown `t`/fields ignored; 64 KiB line cap; 5 malformed lines → close.

## Spike results (2026-09-03, Omarchy + Diamond)

Phase 0(a) is **confirmed on hardware**, so the transport choice stands:

- The Diamond's HID report descriptor contains
  `05 08 19 01 29 05 75 01 95 05 91 02` — usage page LED, usage min NumLock
  through usage max **Kana**, five 1-bit outputs, plus `05 08 75 03 95 01 91 03`
  padding. Report ID `01`. `capabilities/led` is `0x1f` on the keyboard node.
  All three carrier bits (Compose, Kana, Scroll Lock) exist and the
  `[0x01, bits]` hidraw write is the right shape.
- The keyboard appears **twice, on both endpoints at once**: `0003:1D50:615E`
  (USB) and `0005:1D50:615E` (BLE), each with its own hidraw and a
  keyboard + mouse input node (`CONFIG_ZMK_MOUSE=y`). Since ZMK stores the
  indicator byte per endpoint and only the selected one raises the event, the
  daemon must write to every matching device — it does, keying by HID parent.
- Only the keyboard collection carries LEDs; the mouse node reports
  `capabilities/led = 0`, and `openEvdev` rejects it for lacking `EV_LED`.
- The udev rule is installed and both endpoints are writable: `numlock on`
  drives the existing firmware's num-lock listener and toggles vim mode, over
  USB and BLE together.

Phase 0(b) is **confirmed** too, so the LED transport stands and raw HID is not
needed:

- **Typing produces zero `EV_LED` events.** The kernel's LED cache holds 0 for
  Compose/Kana, so libinput's "write all five bits" on every keystroke changes
  nothing and is dropped by `input_get_disposition` before any HID report is
  emitted. A code written over hidraw is therefore undisturbed by ordinary use,
  and there is no re-assert traffic in the steady state.
- **Toggling a lock key produces ~25 events.** That is the real clobber, and it
  is visible on the evdev node, which is exactly the signal
  `internal/leds/linux` re-asserts on.
- This keyboard has no physical Caps Lock (Num Lock lives on the TOGGLES
  layer); any OS-owned lock key demonstrates the same thing.

Still untested: 0(d), whether Ghostty/tmux deliver `FocusGained`/`FocusLost`.

Phase 1 is **running on Omarchy** (systemd user service, stable over 15 h). Two
things the live run corrected:

- **LED capability is not a usable device discriminator.** The daemon happily
  adopted a `Cougar700k Gaming Keyboard` (`060b:700a`), because ordinary
  keyboards also declare all five HID LED indicators. The default filter now
  matches ZMK's `1d50:615e` as well — the same identifiers the shipped udev
  rule uses — with `--any-vendor` to opt out and `--vid`/`--pid` for forks that
  change them. Skipped devices are logged with the reason at debug level.
- **Device ids change across reconnects** (`0005:…0011` → `…0016`), so a
  reconnect is a remove followed by an add. That is handled, and re-assert on
  add is what restores the code.
- **A rewritten unit needs `daemon-reload`**, and with `Type=simple` a command
  issued right after `systemctl restart` reaches the socket before it exists.
  Both made a healthy daemon look dead; `install` now reloads systemd itself
  and `server.Request` retries within its timeout.

Phase 1 is **complete and verified on Omarchy**: `status` reports two devices
(Diamond over USB and BLE, both writable, no unrelated keyboards) and
`frontmost: com.mitchellh.ghostty`, so device discovery, the LED writer and the
Hyprland focus watcher all work from inside the systemd user service.

Phases 2 (Neovim plugin) and 3 (firmware module) are **complete and verified
on Omarchy** (2026-09-09): the module decodes every code, the plugin reports
modes, and the combos announce manual entry/exit through Hyper+Esc / Meh+Esc.
Deviations from the design above, all in the keyboards repo:

- Code 4 does not run `&vim_mode_on`. The listener clears managed layers before
  a code's bindings run, so that layer-morph's guard never fires from a host
  code and it always echoed Hyper+Esc -- which, bound to `set legacy`,
  cancelled the host's own override. Code 4 now sets `VIM_NORMAL` and taps a
  plain Esc (`&vim_mode_on_host`).
- `VIM_LEADER` was removed. Leader handling is host-driven (`raw` while
  pending); legacy apps never had it. Layers above it were renumbered.
- Everything that exists only for legacy mode lives in
  `src/features/vim_legacy.dtsi` (codes 4/7, enter/leave combos, the
  Hyper+Esc / Meh+Esc macros), included last so it can extend the labelled
  `vim_sync` and `combos` nodes. Retiring legacy = delete file + include +
  the two Hyprland binds.
- `tc_cancel` uses `&vim_off` instead of `&vim_mode_off`: a panic key must not
  toggle a sticky `off` override on the host.
- The old `scripts/vimmode/` watchers, `listeners.dtsi` and the
  `ssbb/zmk-listeners` module are gone from the keyboards repo.

Phase 4 (editor integrations) is **implemented, not yet verified on hardware** (2026-09-09): the daemon
follows title changes and combines, per app, title → own client → mode client → legacy; `editors/vscode/`
(companion) and `editors/obsidian/` (plugin) are plain JS with no build step; the protocol gained mode `none`.
IntelliJ stays deferred (not installed). To verify on Omarchy: vscode-neovim + the `window.title` setting +
the companion; the Obsidian plugin copied into the vault; then `zmk-vim-mode status` shows `nvim app=vscode`,
`vscode app=vscode` and `obsidian app=obsidian` clients and `journalctl --user -u zmk-vim-mode` shows the
`tool window focused: Terminal` decisions.

Phase 4 is **verified on Omarchy (2026-09-10)**: VSCode follows editor, terminal, views, quick inputs opened
from mappings, `F1` and the mouse (accessibility bus), the Extensions search box; Obsidian follows modes and
tool windows through its plugin (Wayland app_id is `md.obsidian.Obsidian`). Everything installs with
`zmk-vim-mode install --vscode --obsidian --atspi`. IntelliJ remains deferred (not installed).

Phase 5 is done: installer, README, old watchers deleted, and `doctor` now checks the editor setups (VSCode
title marker, vscode-neovim, companion, lazy spec `vscode = true`; Obsidian plugin per vault) plus a daemon
version older than the CLI. First hardware run of phase 4 (2026-09-10) found two things the plan got wrong —
LazyVim disables plugins inside VSCode unless `vscode = true`, and `${focusedView}` reads `Text Editor`, not
empty, while the editor has focus — both fixed. Next: finish verifying VSCode on the box, then Obsidian, then
phase 6 (macOS) when wanted.

## Phases (Omarchy first)

**Step 0 (first action of the implementation session):** `git init` this directory and commit this plan as
`/Users/rromao/projects/zmk-vim-mode/PLAN.md`, so the implementation session has it without depending on this
conversation. Add `README.md` stub and `.gitignore` (Go build output, `zmk-vim-mode` binary).

0. **Spikes on Omarchy (½–1 day, no new C)** — each with pass/fail:
   a. Bits over BLE: temporarily add `compose {indicator=<HID_USAGE_LED_COMPOSE>; bindings=<&kp A &kp B>;}`
      (+ kana/scroll) to `hid_listeners`, flash Rommana, write `[0x01,0x08]` to hidraw as the user → PASS = `A`
      typed, `[0x01,0x00]` → `B`.
   b. Clobber: keep the bit set and type in Hyprland → PASS = state persists across keystrokes (kernel dedups
      compositor zero-writes); toggle Caps Lock / reconnect BLE → confirm an `EV_LED` echo on eventN and that a
      hidraw rewrite restores state; record timing. FAIL → evdev writes + echo re-assert, or raw HID transport.
   c. Focus events: Ghostty (+tmux `focus-events on`) `:au FocusLost * echo "lost"` → PASS on tab/pane/app switch.
   d. udev: `udevadm test` the rule; check ACL timing (`IN_CREATE` vs `IN_ATTRIB`).
1. **Daemon core, Linux (2–3 days)**: `proto`, `state` (Decide + tests), `server`, `leds/linux` (hidraw write,
   evdev echo, hotplug), `focus/hyprland`, reconciler, `set/status/devices`. Validate the whole pipeline against
   the spike firmware (`set normal` types `A`) before any firmware C.
2. **Neovim plugin (1–2 days)**: transport + effective-state classifier + leader tracking + tests; end-to-end on
   Omarchy with spike firmware, then with the real module.
3. **Firmware module (1–2 days)**: listener with guard/off-delay, DT binding, Kconfig/CMake, keymap node, remove
   old num_lock listener (if agreed), build/flash centrals; validate all 8 codes via `set` (dongle display shows
   layer names).
4. **Editor integrations (2–4 days, §4)**: install `vscode-neovim`, verify our Neovim plugin reports modes
   inside it, add the VSCode companion extension (window focus + terminal hints) and the TTL-based `raw`
   downgrade in the daemon; Obsidian plugin (`vim-mode-change` + `dialog` + `focusin`); `resync` message;
   `set` hotkey bindings for Hyprland (`contrib/hyprland-bind.conf`). IntelliJ only if/when installed.
5. **Install/doctor/docs, Linux (1 day)**: systemd, udev, installer, README, retire old watcher.
6. **macOS backend (2–3 days)**: TCC spike from launchd, `leds/darwin` (IOKit, dedicated writer thread),
   `focus/darwin` (NSWorkspace on `CFRunLoopRun` main thread, `runtime.LockOSThread`), launchd install,
   AX context classifier for legacy apps, retire Hammerspoon watcher.
7. **Polish (optional)**: keyboard→host override via Hyper+Esc/Meh+Esc; X11 (`xset led 4`)/GNOME/KDE focus
   backends; upstream the code-listener to `ssbb/zmk-listeners`; SSH tip (`ssh -R` socket forwarding).

## Verification
- **Unit (`internal/state`)**: frontmost unknown + focused client → client mode (fail open); browser + client →
  off; VSCode + client (no editor client) → legacy; Ghostty + no clients → off; two focused clients → later
  focus seq wins; A focused=false vs B unknown → B; nested tie-break; `focus false` (VimSuspend) → off;
  override + frontmost change → cleared; expired TTL ignored; client disconnect → next client or off; title
  heuristic on/off; Neovide in `gui_nvim_apps`. **Reconciler**: 3 `SetDesired` in 10 ms → one write; same code
  → no write; `Reassert` with desired 4 writes 7; echo ≠ desired → one debounced write, 50/s → rate-limited;
  `Added` → write, error → retries then log. **Server**: 0600/0700, second instance refused, oversized line →
  close, `v:2` hello → error. **Proto** round-trips. **Lua**: every `:h mode()` string → expected coarse/nil.
- **Firmware bench (via `set`)**: all 8 codes; pulse 0 for 20 ms → no layer change; write 2 within 100 ms of a
  local Esc → applied only after the guard; 4→7→0→4 injects exactly one ESC; profile switch Mac↔Linux adopts
  each host's code; `cw` keeps CHANGE until insert.
- **E2E (Omarchy, then Mac)**: dashboard at startup → RAW (letter shortcuts work) → open file → NORMAL; `i`/`Esc`/
  `v`/`:`/`R`; `<leader>` → RAW while which-key is open, `ff` → picker INSERT, `Esc`×2 → NORMAL; `<leader>e` →
  RAW, `a/d/r` reach neo-tree, back → NORMAL; `:Lazy`/`:Mason` → RAW, `q` → NORMAL; `:terminal` + `i` → RAW,
  `<esc><esc>` → NORMAL; rolled `<CR>i` 10–20 ms apart → no dropped keys/stray ESC; switch to browser → OFF,
  back → previous; VSCode editor → LEGACY, its terminal → RAW (phase 4); tmux pane switch, two nvims, nvim in
  `:terminal`; `kill -9 nvim`; daemon stop → keyboard OFF; daemon restart → reconnect < 2 s without OFF
  flicker; BLE reconnect / Caps toggle / sleep-wake → code re-asserted, no ESC; `set raw --ttl 10s` overrides
  then expires; second keyboard on USB simultaneously.

## Manual actions after this work (target)
One-time (Linux): `make install` + one `sudo` for udev; add the lazy.nvim spec line; `set -g focus-events on`;
rebuild + flash each central/dongle; remove the old watcher. One-time (macOS, later): `make install`; grant
Input Monitoring and/or Accessibility if the spike says so. Recurring: none.

## References (read during research)
- `ssbb/zmk-listeners` `main` — module plumbing, DT→bindings extraction, `zmk_behavior_queue_add`.
- zmkfirmware/zmk `app/src/hid_indicators.c`, `app/include/zmk/{hid.h,keymap.h,behavior.h}`, `behavior_queue.c`.
- libinput `src/evdev.c` `evdev_device_led_update`; Hyprland `InputManager.cpp` `updateKeyboardsLeds`,
  `IKeyboard.cpp`; kernel `hid-input.c`, `input.c` `input_get_disposition`, `uhid.c`; BlueZ `hog-lib.c`.
- `hammerspoon/extensions/hid/led.m` (MIT) — IOKit LED write sequence (setledsmac is GPL-2: read, don't copy).
- `/Users/rromao/projects/wayland-setleds/libwayland-setleds.c` (user's, MIT) — evdev probe/ioctl patterns
  (its CLI `-num/-caps` off-forms are broken by `getopt_long`; only 3 LEDs; don't shell out to it).
- `morph-k/rawtalk` — daemon shape (socket → dedupe → single writer), Neovim `vim.uv` client.
- Scripts being replaced: `~/.hammerspoon/zmk-vim-mode-watcher.lua`,
  `~/dotfiles/omarchy/.config/hypr/zmk-vim-mode-watcher.sh`.
