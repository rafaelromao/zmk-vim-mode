#!/usr/bin/env bash
# Phase 0 spikes for zmk-vim-mode, to run on the Omarchy box.
#
# Answers three questions before any firmware C is written:
#   a) do Compose/Kana/Scroll LED bits reach ZMK over BLE?
#   b) does a hidraw-written bit survive Hyprland's per-keystroke LED writes?
#   c) which actions really clobber it, and does the evdev echo report them?
#
# Read-only unless you pass a write subcommand. No sudo needed once the udev
# rule from contrib/udev/ is installed.
set -uo pipefail

VID=${VID:-1d50}
PID=${PID:-615e}
REPORT_ID=1

# LED indicator bits in the ZMK output report (HID usage - 1).
BIT_NUM=1; BIT_CAPS=2; BIT_SCROLL=4; BIT_COMPOSE=8; BIT_KANA=16

die() { echo "error: $*" >&2; exit 1; }

# Emits one tab-separated row per matching evdev node:
#   evnode  hidraw  ledhex  bus  name  hidparent
# ledhex is the last word of capabilities/led ("0" when the node has none).
# A ZMK keyboard exposes several HID collections, so expect more than one row:
# only the keyboard collection carries the LED bits.
find_nodes() {
  local found=0
  for ev in /sys/class/input/event*; do
    [[ -r "$ev/device/id/vendor" ]] || continue
    local v p
    v=$(<"$ev/device/id/vendor"); p=$(<"$ev/device/id/product")
    [[ "${v,,}" == "${VID,,}" && "${p,,}" == "${PID,,}" ]] || continue
    local evname hid raw name bus led
    evname=/dev/input/$(basename "$ev")
    name=$(<"$ev/device/name" 2>/dev/null) || name="?"
    bus=$(<"$ev/device/id/bustype")
    led=$(<"$ev/device/capabilities/led" 2>/dev/null) || led=0
    led=${led##* }
    hid=$(readlink -f "$ev/device/device" 2>/dev/null || true)
    raw=""
    if [[ -n "$hid" ]]; then
      for hr in /sys/class/hidraw/hidraw*; do
        [[ "$(readlink -f "$hr/device")" == "$hid" ]] && raw=/dev/$(basename "$hr") && break
      done
    fi
    printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$evname" "${raw:--}" "$led" "$bus" "$name" "$(basename "${hid:--}")"
    found=1
  done
  return $((1 - found))
}

# The node that actually carries the LED bits: capabilities/led must have
# compose (bit 3) and kana (bit 4).
led_node() {
  local row
  while IFS=$'\t' read -r evnode raw led bus name hid; do
    local v=$((16#${led:-0}))
    if (( (v >> 3) & 1 )) && (( (v >> 4) & 1 )); then
      printf '%s\t%s\n' "$evnode" "$raw"
      return 0
    fi
    row=1
  done < <(find_nodes)
  [[ -n "${row:-}" ]] && return 1
  return 2
}

cmd_find() {
  local any=0 rows=""
  while IFS=$'\t' read -r evnode raw led bus name hid; do
    any=1
    rows+=$(printf '%-18s %-13s %-7s %-5s %s\n' "$evnode" "$raw" "$led" "$bus" "$name")$'\n'
  done < <(find_nodes)
  (( any )) || die "no device $VID:$PID found. Is the keyboard connected/paired?"
  printf '%-18s %-13s %-7s %-5s %s\n' "event node" "hidraw" "led" "bus" "name"
  printf '%s' "$rows"
  echo
  echo "bus=3 is USB, bus=5 is Bluetooth LE. ZMK keeps indicator state per endpoint,"
  echo "so a result on one transport says nothing about the other: test both."
  echo "Several rows are normal (keyboard, consumer, mouse collections); only the"
  echo "row with a non-zero led column receives the indicator report."
  echo "A '-' hidraw column means no HID parent resolved; writes fall back to evdev."
}

cmd_caps() {
  local any=0
  while IFS=$'\t' read -r evnode raw led bus name hid; do
    any=1
    local v=$((16#${led:-0}))
    printf '%s (%s)\n  capabilities/led = %s\n' "$evnode" "$name" "$led"
    local missing=0
    for pair in "0:numlock" "1:capslock" "2:scrolllock" "3:compose" "4:kana"; do
      local n=${pair%%:*} label=${pair##*:}
      if (( (v >> n) & 1 )); then
        echo "  has $label"
      else
        echo "  MISSING $label"
        missing=1
      fi
    done
    if (( v == 0 )); then
      echo "  → not the keyboard collection; ignore this node"
    elif (( missing )); then
      echo "  → some indicators are absent"
    else
      echo "  → this node carries the 3-bit code"
    fi
    echo
  done < <(find_nodes)
  (( any )) || die "no device $VID:$PID found"
  echo "compose+kana+scrolllock must all be present on at least one node."
  echo "If no node has them, CONFIG_ZMK_HID_INDICATORS is not enabled in the firmware."
}

# Write a raw indicator bitmap to the keyboard's LED output report.
write_bits() {
  local bits=$1 what=$2 raw
  raw=$(led_node | cut -f2)
  case $? in
    1) die "found the keyboard but no node exposes compose+kana LEDs. Run '$0 caps'." ;;
    2) die "no device $VID:$PID found. Is the keyboard connected?" ;;
  esac
  [[ -n "$raw" && "$raw" != "-" ]] || die "no hidraw node paired with the LED node; check the udev rule"
  [[ -w "$raw" ]] || die "$raw is not writable. Install contrib/udev/60-zmk-vim-mode.rules, reload udev, then reconnect the keyboard."
  printf "writing %s: report 0x%02x 0x%02x to %s\n" "$what" "$REPORT_ID" "$bits" "$raw"
  printf "\\x$(printf %02x "$REPORT_ID")\\x$(printf %02x "$bits")" > "$raw" \
    && echo "ok" || die "write failed"
}

# write <code>  — code 0..7 (b0 compose, b1 kana, b2 scroll)
cmd_write() {
  local code=${1:-}
  [[ "$code" =~ ^[0-7]$ ]] || die "usage: $0 write <0-7>"
  local bits=0
  (( code & 1 )) && bits=$((bits | BIT_COMPOSE))
  (( code & 2 )) && bits=$((bits | BIT_KANA))
  (( code & 4 )) && bits=$((bits | BIT_SCROLL))
  write_bits "$bits" "code $code"
}

# numlock <on|off> — validate the whole write path against the CURRENT firmware,
# which already maps NUM LOCK to vim mode. No flashing required.
cmd_numlock() {
  case "${1:-}" in
    on) write_bits $BIT_NUM "num lock on (expect vim mode ON: an Esc is sent and the NORMAL layer activates)" ;;
    off) write_bits 0 "num lock off (expect vim mode OFF)" ;;
    *) die "usage: $0 numlock <on|off>" ;;
  esac
}

cmd_watch() {
  local ev
  ev=$(find_nodes | head -1 | cut -f1) || die "device not found"
  command -v evtest >/dev/null || die "evtest not installed (sudo pacman -S evtest)"
  echo "watching $ev for EV_LED events; toggle Caps Lock, type, switch workspaces."
  echo "Every line here is the kernel telling all readers that an LED changed —"
  echo "this is what the daemon uses to notice a clobber. Ctrl-C to stop."
  evtest --grab=0 "$ev" 2>/dev/null | grep --line-buffered "EV_LED" || true
}

cmd_spike_a() {
  cat <<'EOF'
Spike (a): do the carrier bits reach ZMK?

Step 0 — no flashing needed. Your current firmware already maps NUM LOCK to
vim mode (the num_lock node in src/features/vim.dtsi), so the whole write path
can be proven right now:

       ./scripts/spike-linux.sh numlock on    # expect: vim mode ON (an Esc is
                                              # typed, the NORMAL layer engages)
       ./scripts/spike-linux.sh numlock off   # expect: vim mode OFF

PASS: host → hidraw → kernel → HID output report → ZMK all work on this
transport. Only the choice of *which* bits remains to be proven.
FAIL: run `caps`, then check the udev rule and that the write reported "ok".

Step 1 — prove the three carrier bits. In ~/projects/keyboards/src/features/
vim.dtsi, temporarily add inside the existing hid_listeners node:

       compose_probe { indicator = <HID_USAGE_LED_COMPOSE>; bindings = <&kp A &kp B>; };
       kana_probe    { indicator = <HID_USAGE_LED_KANA>;    bindings = <&kp C &kp D>; };
       scroll_probe  { indicator = <HID_USAGE_LED_SCROLL_LOCK>; bindings = <&kp E &kp F>; };

Build and flash the central/dongle, then in a text editor:

       ./scripts/spike-linux.sh write 1   # expect: A     (compose on)
       ./scripts/spike-linux.sh write 0   # expect: B     (compose off)
       ./scripts/spike-linux.sh write 2   # expect: C
       ./scripts/spike-linux.sh write 4   # expect: E

PASS: the 3-bit channel works and the design is viable with no new firmware C.
FAIL: check `caps` first.

Step 2 — repeat on the other transport. `find` shows bus=3 for USB and bus=5
for Bluetooth LE. ZMK stores the indicator byte per endpoint, so a pass over
USB does not imply a pass over BLE. Switch the keyboard's output (&out OUT_BLE
/ the BLE profile paired with this machine) and run step 1 again.
EOF
}

cmd_spike_b() {
  cat <<'EOF'
Spike (b): does a written bit survive Hyprland's per-keystroke LED writes?

Hyprland pushes LED state to every keyboard on every key and modifier event,
and libinput writes all five indicator bits, zeroing ours. The bet is that the
kernel's own LED cache stays 0 for Compose/Kana, so those writes change nothing
and are dropped before they reach the keyboard (input_get_disposition).

1. ./scripts/spike-linux.sh write 1        (probe firmware types A)
2. Type a paragraph in any window, switch workspaces, move the mouse.
3. ./scripts/spike-linux.sh write 0        (expect B)

PASS: exactly one A at step 1 and one B at step 3. Nothing in between, i.e.
typing did not clobber the bit.
FAIL (B appears while typing): the kernel is echoing our bits back. The daemon
still recovers via the evdev echo path, but re-assert traffic will be constant;
consider the raw-HID transport instead (PLAN.md, "Alternatives rejected").

Then check the real clobbers, in another terminal:
   ./scripts/spike-linux.sh watch
and toggle Caps Lock, unplug/replug or re-pair the keyboard. Each of those
should print EV_LED lines: that is the signal the daemon re-asserts on.
EOF
}

usage() {
  cat <<EOF
usage: $0 <command>

  find          list the ZMK keyboard's evdev and hidraw nodes
  caps          check which node exposes compose/kana/scrolllock LEDs
  numlock on|off  toggle vim mode through the CURRENT firmware's num-lock
                  listener — proves the write path without flashing anything
  write N       write mode code N (0-7) as a HID output report
  watch         print EV_LED echoes (needs evtest) — shows real clobbers
  spike-a       instructions for spike (a): bits reach ZMK
  spike-b       instructions for spike (b): bits survive Hyprland

Suggested order: find → caps → numlock on/off → spike-a → spike-b

Environment: VID=$VID PID=$PID
EOF
}

case "${1:-}" in
  find) cmd_find ;;
  caps) cmd_caps ;;
  numlock) shift; cmd_numlock "$@" ;;
  write) shift; cmd_write "$@" ;;
  watch) cmd_watch ;;
  spike-a) cmd_spike_a ;;
  spike-b) cmd_spike_b ;;
  *) usage; exit 2 ;;
esac
