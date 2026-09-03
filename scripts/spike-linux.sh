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

find_nodes() {
  local found=0
  for ev in /sys/class/input/event*; do
    [[ -r "$ev/device/id/vendor" ]] || continue
    local v p
    v=$(<"$ev/device/id/vendor"); p=$(<"$ev/device/id/product")
    [[ "$v" == "$VID" && "$p" == "$PID" ]] || continue
    local evname hid raw name bus
    evname=/dev/input/$(basename "$ev")
    name=$(<"$ev/device/name" 2>/dev/null) || name="?"
    bus=$(<"$ev/device/id/bustype")
    hid=$(readlink -f "$ev/device/device" 2>/dev/null || true)
    raw=""
    if [[ -n "$hid" ]]; then
      for hr in /sys/class/hidraw/hidraw*; do
        [[ "$(readlink -f "$hr/device")" == "$hid" ]] && raw=/dev/$(basename "$hr") && break
      done
    fi
    printf '%s\t%s\t%s\tbus=%s\t%s\n' "$evname" "${raw:--}" "$name" "$bus" "$(basename "${hid:-–}")"
    found=1
  done
  return $((1 - found))
}

cmd_find() {
  echo "event node      hidraw       name                    bus   hid parent"
  find_nodes || die "no device $VID:$PID found. Is the keyboard connected/paired?"
  echo
  echo "bus=3 is USB, bus=5 is Bluetooth LE."
  echo "A missing hidraw column means no HID parent was resolved; writes fall back to evdev."
}

cmd_caps() {
  local ev
  ev=$(find_nodes | head -1 | cut -f1) || die "device not found"
  [[ -n "$ev" ]] || die "device not found"
  echo "checking LED capabilities of $ev"
  local sys="/sys/class/input/$(basename "$ev")/device/capabilities/led"
  local bits; bits=$(<"$sys")
  echo "  capabilities/led = $bits"
  # LED_COMPOSE=3, LED_KANA=4 must be present for the 3-bit code to work.
  local v=$((16#${bits##* }))
  for pair in "0:numlock" "1:capslock" "2:scrolllock" "3:compose" "4:kana"; do
    local n=${pair%%:*} label=${pair##*:}
    if (( (v >> n) & 1 )); then echo "  has $label"; else echo "  MISSING $label"; fi
  done
  echo
  echo "compose+kana+scrolllock must all be present."
  echo "If they are not, CONFIG_ZMK_HID_INDICATORS is off in the firmware."
}

# write <code>  — code 0..7 (b0 compose, b1 kana, b2 scroll)
cmd_write() {
  local code=${1:-} raw
  [[ "$code" =~ ^[0-7]$ ]] || die "usage: $0 write <0-7>"
  raw=$(find_nodes | head -1 | cut -f2)
  [[ -n "$raw" && "$raw" != "-" ]] || die "no hidraw node; check the udev rule"
  [[ -w "$raw" ]] || die "$raw is not writable. Install contrib/udev/60-zmk-vim-mode.rules, reload udev, reconnect the keyboard."
  local bits=0
  (( code & 1 )) && bits=$((bits | BIT_COMPOSE))
  (( code & 2 )) && bits=$((bits | BIT_KANA))
  (( code & 4 )) && bits=$((bits | BIT_SCROLL))
  printf "writing code %d (report 0x%02x 0x%02x) to %s\n" "$code" "$REPORT_ID" "$bits" "$raw"
  printf "\\x$(printf %02x $REPORT_ID)\\x$(printf %02x $bits)" > "$raw" \
    && echo "ok" || die "write failed"
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
Spike (a): do the three carrier bits reach ZMK over BLE?

1. In ~/projects/keyboards/src/features/vim.dtsi, temporarily add inside the
   existing hid_listeners node:

       compose_probe { indicator = <HID_USAGE_LED_COMPOSE>; bindings = <&kp A &kp B>; };
       kana_probe    { indicator = <HID_USAGE_LED_KANA>;    bindings = <&kp C &kp D>; };
       scroll_probe  { indicator = <HID_USAGE_LED_SCROLL_LOCK>; bindings = <&kp E &kp F>; };

2. Build and flash the central/dongle, then open a text editor and run:

       ./scripts/spike-linux.sh write 1   # expect: A     (compose on)
       ./scripts/spike-linux.sh write 0   # expect: B     (compose off)
       ./scripts/spike-linux.sh write 2   # expect: C
       ./scripts/spike-linux.sh write 4   # expect: E

PASS: the letters appear. The 3-bit channel works over BLE and the whole
design is viable with zero new firmware C.
FAIL: check `caps` output first, then whether the keyboard is on the BLE
profile paired with this machine (indicators are stored per endpoint).
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

  find      list the ZMK keyboard's evdev and hidraw nodes
  caps      check that the firmware exposes compose/kana/scrolllock LEDs
  write N   write mode code N (0-7) as a HID output report
  watch     print EV_LED echoes (needs evtest) — shows real clobbers
  spike-a   instructions for spike (a): bits reach ZMK over BLE
  spike-b   instructions for spike (b): bits survive Hyprland

Environment: VID=$VID PID=$PID
EOF
}

case "${1:-}" in
  find) cmd_find ;;
  caps) cmd_caps ;;
  write) shift; cmd_write "$@" ;;
  watch) cmd_watch ;;
  spike-a) cmd_spike_a ;;
  spike-b) cmd_spike_b ;;
  *) usage; exit 2 ;;
esac
