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
    # The name lives on the inputN parent; fall back to the class dir and to
    # the HID parent's own name, so a device is never reported as anonymous.
    name=$(cat "$ev/device/name" 2>/dev/null || cat "$ev/../name" 2>/dev/null || true)
    [[ -n "$name" ]] || name="<no name>"
    bus=$(cat "$ev/device/id/bustype" 2>/dev/null || echo "?")
    led=$(cat "$ev/device/capabilities/led" 2>/dev/null || echo 0)
    led=${led##* }
    [[ -n "$led" ]] || led=0
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

# sysfs prints id/bustype as zero-padded hex, so compare numerically.
bus_name() {
  case $((16#${1:-0})) in
    3) printf 'USB' ;;
    5) printf 'BLE' ;;
    *) printf 'bus %s' "$1" ;;
  esac
}

# LED capability bitmap of one event node, read straight from sysfs.
# Deliberately not taken from find_nodes' output: relying on field position
# across functions is what made `caps` report the bustype as the LED mask.
led_bits_of() {
  local base v
  base=$(basename "$1")
  v=$(cat "/sys/class/input/$base/device/capabilities/led" 2>/dev/null || echo 0)
  v=${v##* }
  printf '%d' "$((16#${v:-0}))"
}

cmd_find() {
  local any=0 rows=""
  while IFS=$'\t' read -r evnode raw led bus name hid; do
    any=1
    rows+=$(printf '%-18s %-13s %-7x %-5s %s\n' "$evnode" "$raw" "$(led_bits_of "$evnode")" "$bus" "$name")$'\n'
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
    local v
    v=$(led_bits_of "$evnode")
    printf '%s (%s)\n  capabilities/led = %x\n' "$evnode" "$name" "$v"
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

# Write a raw indicator bitmap to EVERY matching keyboard.
#
# The same keyboard usually appears on more than one endpoint (USB and BLE at
# once is normal), and ZMK stores the indicator byte per endpoint while only
# the currently selected one raises the event. Writing just the first match is
# a coin flip, so write them all — this is what the daemon does too.
#
# $3 (optional) is the LED-capability mask a node must have; it defaults to
# compose|kana because that is what the mode code rides on.
write_bits() {
  local bits=$1 what=$2 want=${3:-$(( (1<<3) | (1<<4) ))}
  local any=0 candidates=0 wrote=0 failed=0 seen=""

  printf 'writing %s: report 0x%02x 0x%02x\n' "$what" "$REPORT_ID" "$bits"
  while IFS=$'\t' read -r evnode raw led bus name hid; do
    any=1
    local v
    v=$(led_bits_of "$evnode")
    (( (v & want) == want )) || continue
    candidates=1
    if [[ -z "$raw" || "$raw" == "-" ]]; then
      printf '  skip  %-14s %s (no hidraw node paired)\n' "$evnode" "$name"
      continue
    fi
    case " $seen " in *" $raw "*) continue ;; esac   # keyboard+mouse share one hidraw
    seen+=" $raw"
    local transport
    transport=$(bus_name "$bus")
    if [[ ! -w "$raw" ]]; then
      printf '  FAIL  %-14s %s [%s] not writable\n' "$raw" "$name" "$transport"
      failed=$((failed + 1))
      continue
    fi
    if printf "\\x$(printf %02x "$REPORT_ID")\\x$(printf %02x "$bits")" > "$raw" 2>/dev/null; then
      printf '  ok    %-14s %s [%s]\n' "$raw" "$name" "$transport"
      wrote=$((wrote + 1))
    else
      printf '  FAIL  %-14s %s [%s] write error\n' "$raw" "$name" "$transport"
      failed=$((failed + 1))
    fi
  done < <(find_nodes)

  (( any )) || die "no device $VID:$PID found. Is the keyboard connected?"
  (( candidates )) || die "found the keyboard, but no node exposes the required LEDs. Run '$0 caps' / '$0 dump'."
  if (( wrote == 0 )); then
    die "no endpoint could be written. Install contrib/udev/60-zmk-vim-mode.rules, reload udev, then reconnect the keyboard."
  fi
  if (( failed )); then
    echo "  ($wrote written, $failed failed — ZMK only acts on the selected endpoint," >&2
    echo "   so a failure here may be why nothing changed)" >&2
  fi
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
# which already maps NUM LOCK to vim mode. No flashing required, and it only
# needs LED_NUML, so it works even when compose/kana are missing.
cmd_numlock() {
  local numonly=$(( 1 << 0 ))
  case "${1:-}" in
    on) write_bits $BIT_NUM "num lock on (expect vim mode ON: an Esc is sent and the NORMAL layer activates)" "$numonly" ;;
    off) write_bits 0 "num lock off (expect vim mode OFF)" "$numonly" ;;
    *) die "usage: $0 numlock <on|off>" ;;
  esac
}

# Ground truth: what the firmware actually declares, straight from the HID
# report descriptor, plus the raw sysfs values behind `caps`.
cmd_dump() {
  local any=0
  while IFS=$'\t' read -r evnode raw led bus name hid; do
    any=1
    local base sys
    base=$(basename "$evnode")
    sys=$(readlink -f "/sys/class/input/$base/device" 2>/dev/null)
    echo "=============================================================="
    echo "$evnode  ->  $sys"
    echo "  hidraw:      $raw"
    echo "  hid parent:  $hid"
    for f in name phys uniq; do
      printf '  %-11s %s\n' "$f:" "$(cat "$sys/$f" 2>/dev/null || echo '<unreadable>')"
    done
    for f in ev led key rel abs; do
      printf '  cap/%-8s %s\n' "$f:" "$(cat "$sys/capabilities/$f" 2>/dev/null || echo '-')"
    done
    local ledv=$((16#${led:-0}))
    printf '  led bits:    0x%x ->' "$ledv"
    for pair in "0:num" "1:caps" "2:scroll" "3:compose" "4:kana"; do
      (( (ledv >> ${pair%%:*}) & 1 )) && printf ' %s' "${pair##*:}"
    done
    echo
  done < <(find_nodes)
  (( any )) || die "no device $VID:$PID found"

  echo
  echo "=============================================================="
  echo "HID report descriptors"
  echo "=============================================================="
  local seen=""
  while IFS=$'\t' read -r evnode raw led bus name hid; do
    [[ -n "$raw" && "$raw" != "-" ]] || continue
    case " $seen " in *" $raw "*) continue ;; esac
    seen+=" $raw"
    local rd="/sys/class/hidraw/$(basename "$raw")/device/report_descriptor"
    echo
    echo "--- $raw ($hid)"
    if [[ ! -r "$rd" ]]; then
      echo "    $rd not readable"
      continue
    fi
    local hex
    hex=$(od -An -tx1 -v "$rd" | tr -s ' ' | tr -d '\n' | sed 's/^ //')
    echo "    bytes: $(wc -c < "$rd")"
    echo "$hex" | fold -w 72 | sed 's/^/    /'
    echo
    # Usage Page (LED) is "05 08". What follows tells us how many indicator
    # bits the firmware declares as an OUTPUT report.
    if [[ "$hex" == *"05 08"* ]]; then
      echo "    LED usage page (05 08) IS present:"
      echo "$hex" | grep -o '05 08\( [0-9a-f][0-9a-f]\)\{0,14\}' | sed 's/^/      /'
      echo "      expect: 19 01 (usage min NumLock) 29 05 (usage max Kana)"
      echo "              75 01 (1 bit) 95 05 (5 of them) 91 02 (output,var,abs)"
    else
      echo "    LED usage page (05 08) IS ABSENT."
      echo "    => this firmware declares no LED indicators at all"
      echo "       (CONFIG_ZMK_HID_INDICATORS is not enabled in the build that is flashed)"
    fi
  done < <(find_nodes)
  echo
  echo "If 05 08 is present but followed by '29 03' instead of '29 05', the"
  echo "firmware declares only 3 indicators (num/caps/scroll) and Compose+Kana"
  echo "are unavailable: the code must then fit in the bits that do exist."
}

cmd_watch() {
  command -v evtest >/dev/null || die "evtest not installed (sudo pacman -S evtest)"
  local want=$(( (1<<3) | (1<<4) )) ev="" others=""
  while IFS=$'\t' read -r evnode raw led bus name hid; do
    local v
    v=$(led_bits_of "$evnode")
    (( (v & want) == want )) || continue
    if [[ -z "$ev" ]]; then
      ev=$evnode
      echo "watching $evnode  ($name, $(bus_name "$bus"))"
    else
      others+="  $evnode ($name, $(bus_name "$bus"))"$'\n'
    fi
  done < <(find_nodes)
  [[ -n "$ev" ]] || die "no node exposes the carrier LEDs. Run '$0 caps' / '$0 dump'."
  if [[ -n "$others" ]]; then
    echo "other endpoints (evtest watches one at a time; rerun in another terminal):"
    printf '%s' "$others"
  fi
  [[ -r "$ev" ]] || die "$ev is not readable. Install contrib/udev/60-zmk-vim-mode.rules, reload udev, then reconnect the keyboard."
  echo
  echo "Each line below means the kernel changed LED state and therefore sent a"
  echo "HID output report -- which zeroes our bits. Silence while typing is the"
  echo "PASS condition; lines on Caps Lock or reconnect are expected and are what"
  echo "the daemon re-asserts on. Ctrl-C to stop."
  echo
  # Never pass --grab: it is EVIOCGRAB, which takes the keyboard exclusively
  # and would cut input off from the rest of the session. Match only real
  # events ("Event: ... EV_LED"), not evtest's capability header, which also
  # contains the string EV_LED. stderr is left alone so evtest can complain.
  evtest "$ev" | grep --line-buffered -E '^Event:.*EV_LED'
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
and are dropped before they ever reach the keyboard (input_get_disposition).

The decisive observation needs NO firmware change. An EV_LED event on the
evdev node means the kernel's LED state actually changed, which means it sent
a HID output report to the keyboard -- and that report carries zeros in our
bits. So:

    no EV_LED while typing  =>  our bits are never overwritten  =>  PASS

Method 1 (no flashing, no daemon) -- needs evtest:

    ./spike-linux.sh watch
    # then type a paragraph, switch workspaces, move the mouse

PASS: the stream stays silent while you type.
FAIL: an EV_LED line per keystroke.

Now provoke the real clobbers, with `watch` still running: toggle Caps Lock,
then re-pair or replug the keyboard. Each SHOULD print EV_LED lines -- that is
the signal the daemon re-asserts on, so seeing them here is the good outcome.

Method 2 (exercises the real code) -- run the daemon itself:

    zmk-vim-mode daemon --log-level debug
    zmk-vim-mode set normal      # in another terminal: writes the compose bit
    # type a paragraph

Watch the daemon log. "led echo" lines during typing mean the kernel is
rewriting our bits; silence means it is not. Either way the daemon recovers,
but constant re-assert traffic on a BLE link is worth avoiding.

Method 3 (definitive, needs the probe firmware from spike-a):

    ./spike-linux.sh write 1     # probe firmware types A
    # type a paragraph
    ./spike-linux.sh write 0     # expect B

PASS: exactly one A and one B, nothing in between.

If this FAILS, the daemon still works via the evdev echo path, but re-assert
traffic will be constant and raw HID becomes the better transport
(see PLAN.md, "Alternatives rejected").
EOF
}

usage() {
  cat <<EOF
usage: $0 <command>

  find          list the ZMK keyboard's evdev and hidraw nodes
  caps          check which node exposes compose/kana/scrolllock LEDs
  dump          full diagnostic: sysfs values + HID report descriptor
                (says definitively what the firmware declares)
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
  dump) cmd_dump ;;
  numlock) shift; cmd_numlock "$@" ;;
  write) shift; cmd_write "$@" ;;
  watch) cmd_watch ;;
  spike-a) cmd_spike_a ;;
  spike-b) cmd_spike_b ;;
  *) usage; exit 2 ;;
esac
