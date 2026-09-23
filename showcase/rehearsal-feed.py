#!/usr/bin/env python3
"""The rehearsal's feed for zmk-layer-hud's pages: the keyboard's own signal channel, plus
the keys ydotool injects.

    python3 showcase/rehearsal-feed.py [--no-keyboard] [--device ydotool] [--debug]

A take needs nothing of this: zmk-layer-hud reads the Diamond's HID reports and draws exactly
what it did (`bash showcase/hud.sh`). A rehearsal types with ydotool, which writes to
/dev/uinput and never reaches the keyboard, so those keys are invisible to that feed. This one
runs zmk-layer-hud's own reader -- the keymap and its live reload, the device name, and the
layers the keyboard really is on, which follow `zmk-vim-mode set ...` because the daemon writes
the mode to the keyboard -- and adds the injected keys read off ydotool's virtual device. The
pages then light them against the real layer stack, exactly as they light real typing.
Arrows, digits, and symbols are drawn through their nav/numbers/symbols layers, never as combos.
Letters on the Diamond's secondary alpha layer are modelled as one-shot alpha2 activations,
including the adaptive h|v/v|h magic key. Literal text entered in Vim's insert, replace, or
command-line modes uses the same layer rules as shell and editor text. The release puts the
keyboard's own layers back at once (a same-valued heartbeat re-asserts nothing, so the restore is
the correction, not a speedup).

Everything about the protocol, the keymap and the layers comes from zmk-layer-hud
($ZMK_LAYER_HUD, default ~/projects/zmk-layer-hud); this file only adds the evdev half. Port:
$ZMKHUD_PORT, default 8767, so a real HUD on 8766 is undisturbed.

Needs: that project's venv (pyserial, keymap-drawer, websockets, bleak) and python-evdev,
plus read access to the injected device (the `input` group). The venv needs no hidapi on
Linux: the feed reads the keyboard's signal tty plus /dev/hidrawN directly.
"""

import argparse
import asyncio
import os
import sys
from pathlib import Path

HUD = Path(os.environ.get("ZMK_LAYER_HUD", Path.home() / "projects/zmk-layer-hud")).expanduser()
if not (HUD / "host" / "hudfeed.py").is_file():
    sys.exit(f"No zmk-layer-hud checkout at {HUD}: clone it or set ZMK_LAYER_HUD")
sys.path.insert(0, str(HUD / "host"))

try:
    import hudfeed  # Hub, Feed: the protocol, the keymap and the keyboard reader
except ImportError as e:
    sys.exit(f"cannot import zmk-layer-hud's feed ({e}); run `make venv` in {HUD}")

try:
    import evdev
    from evdev import ecodes as E
except ImportError:
    sys.exit("python-evdev is required to see the injected keys: sudo pacman -S python-evdev")

from typist import (ALPHA2_CHARS, NUMBER_LAYER_CHARS, SYMBOL_LAYER_CHARS, VOWELS,
                    is_vowel, remember_char, uses_alpha2)

PORT = int(os.environ.get("ZMKHUD_PORT", "8767"))
# ydotoold's uinput device, by name. `libinput list-devices` names yours if this misses.
DEVICE = os.environ.get("ZMKHUD_INJECT_DEVICE", "ydotool")

# US layout: evdev key -> (character, shifted character). The pages resolve characters against
# the keymap-drawer legends, so this only has to match what the host layout produces.
CHARS = {
    **{getattr(E, f"KEY_{c}"): (c.lower(), c) for c in "ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
    E.KEY_1: ("1", "!"), E.KEY_2: ("2", "@"), E.KEY_3: ("3", "#"), E.KEY_4: ("4", "$"),
    E.KEY_5: ("5", "%"), E.KEY_6: ("6", "^"), E.KEY_7: ("7", "&"), E.KEY_8: ("8", "*"),
    E.KEY_9: ("9", "("), E.KEY_0: ("0", ")"), E.KEY_MINUS: ("-", "_"), E.KEY_EQUAL: ("=", "+"),
    E.KEY_LEFTBRACE: ("[", "{"), E.KEY_RIGHTBRACE: ("]", "}"), E.KEY_BACKSLASH: ("\\", "|"),
    E.KEY_SEMICOLON: (";", ":"), E.KEY_APOSTROPHE: ("'", '"'), E.KEY_GRAVE: ("`", "~"),
    E.KEY_COMMA: (",", "<"), E.KEY_DOT: (".", ">"), E.KEY_SLASH: ("/", "?"),
}
CONTROL_CHARS = {E.KEY_SPACE: " ", E.KEY_ENTER: "\r", E.KEY_ESC: "\x1b"}
# Named keys, spelled as zmk-layer-hud's own NAMED table spells them (the pages share one).
NAMED = {
    E.KEY_SPACE: "space", E.KEY_ENTER: "return", E.KEY_ESC: "escape", E.KEY_BACKSPACE: "delete",
    E.KEY_DELETE: "forwarddelete", E.KEY_TAB: "tab", E.KEY_LEFT: "left", E.KEY_RIGHT: "right",
    E.KEY_UP: "up", E.KEY_DOWN: "down", E.KEY_HOME: "home", E.KEY_END: "end",
    E.KEY_PAGEUP: "pageup", E.KEY_PAGEDOWN: "pagedown",
    **{getattr(E, f"KEY_F{n}"): f"f{n}" for n in range(1, 13)},
}
MODS = {
    E.KEY_LEFTSHIFT: "shift", E.KEY_RIGHTSHIFT: "shift", E.KEY_LEFTCTRL: "ctrl",
    E.KEY_RIGHTCTRL: "ctrl", E.KEY_LEFTALT: "alt", E.KEY_RIGHTALT: "alt",
    E.KEY_LEFTMETA: "cmd", E.KEY_RIGHTMETA: "cmd",
}
# Character layers and navigation use held layers, never combos. The rehearsal
# models the same thumbs the typist uses for literal text on camera.
NAV_CODES = {E.KEY_UP, E.KEY_DOWN, E.KEY_LEFT, E.KEY_RIGHT,
             E.KEY_HOME, E.KEY_END, E.KEY_PAGEUP, E.KEY_PAGEDOWN}
NAV_DRAWERS = {"nav"}
NUMBER_DRAWERS = {"numbers"}
SYMBOL_DRAWERS = {"symbols"}
ALPHA2_THUMB_IDX = 22
SHIFT_THUMB_IDX = 23
NUMBERS_THUMB_IDX = 21
SYMBOLS_THUMB_IDX = 22
TEXT_ENTRY_VIM_LAYERS = {"insert", "replace", "cmdline"}


class InjectedKeys:
    """Reads one evdev device and emits zmk-layer-hud key messages for what it types."""

    def __init__(self, emit, log, match=DEVICE, keymap=None, true_layers=None, layers=None):
        self.emit, self.log, self.match = emit, log, match
        self.held = {"cmd": 0, "ctrl": 0, "alt": 0, "shift": 0}
        # The keymap message (for drawer -> ZMK layer ids) and the keyboard's
        # own current layer ids (to put back after an emulated hold). Both are
        # best effort: without them the keys still light, just with no banner.
        self.keymap = keymap or (lambda: None)
        self.layer_state = layers or true_layers or (lambda: None)
        self.layer_restore = {}
        self.layer_activator = {}
        self.previous_char = None

    def flags(self):
        return {k: v > 0 for k, v in self.held.items()} | {"fn": False}

    def message(self, code, value):
        """One evdev event -> a key message (value: 1 down, 0 up, 2 repeat)."""
        if code in MODS:
            name = MODS[code]
            self.held[name] = max(0, self.held[name] + (1 if value == 1 else -1 if value == 0 else 0))
            return {"kind": "key", "type": "flagsChanged", "name": "", "chars": "", "code": 0,
                    "flags": self.flags(), "repeat": False, "synthetic": True}
        flags = self.flags()
        chars = CONTROL_CHARS.get(code, "")
        if not chars and code in CHARS:
            chars = CHARS[code][1 if flags["shift"] else 0]
        name = NAMED.get(code) or (chars if chars and chars != " " else
                                   E.KEY.get(code, str(code)).replace("KEY_", "").lower())
        return {"kind": "key", "type": "keyDown" if value else "keyUp", "name": name,
                "chars": chars, "code": code, "flags": flags, "repeat": value == 2,
                "synthetic": True}

    def drawer_ids(self, drawers, preferred_names=()):
        """ZMK layer ids drawn with one of these drawers, from the keymap message."""
        km = self.keymap() or {}
        found = []
        for lid, info in (km.get("zmk_layers") or {}).items():
            if isinstance(info, dict) and info.get("drawer") in drawers:
                try:
                    found.append((info.get("name"), int(info.get("id", lid))))
                except (TypeError, ValueError):
                    pass
        preferred = [lid for name, lid in found if name in preferred_names]
        return sorted(preferred or [lid for _, lid in found])

    def current_layers(self):
        """The last known real layer stack, or None before the feed has reported one."""
        ids = self.layer_state()
        if ids is None:
            return None
        return list(dict.fromkeys(int(i) for i in ids))

    def stack_with(self, base, drawers, preferred_names=()):
        ids = list(base or [])
        for layer in self.drawer_ids(drawers, preferred_names):
            if layer not in ids:
                ids.append(layer)
        return ids

    def command_layers_active(self):
        """Whether Vim is interpreting command keys rather than literal text."""
        km = self.keymap() or {}
        current = self.current_layers() or []
        layers = [(km.get("zmk_layers") or {}).get(str(i), {}) for i in current]
        vim_layers = [layer for layer in layers if layer.get("drawer") == "vim"]
        if any(str(layer.get("name", "")).casefold() in TEXT_ENTRY_VIM_LAYERS
               for layer in vim_layers):
            return False
        return bool(vim_layers)

    def position_for_idx(self, idx):
        """Reverse the keymap's ZMK-position -> drawer-index map for a thumb flash."""
        km = self.keymap() or {}
        for pos, drawer_idx in (km.get("positions") or {}).items():
            try:
                if int(drawer_idx) == idx:
                    return int(pos)
            except (TypeError, ValueError):
                pass
        return None

    def remember_char(self, chars):
        self.previous_char = remember_char(chars)

    def alpha2_drawer(self, chars):
        """Return the drawer the Diamond owner would use for this typed character."""
        if (self.command_layers_active() and self.has_plain_vim_key(chars)):
            return None
        if not uses_alpha2(chars, self.previous_char):
            return None
        return "shifted2" if chars.isalpha() and chars.isupper() else "alpha2"

    def has_plain_vim_key(self, chars):
        """Whether Vim has a single-key binding for this command on its command drawer."""
        keys = (self.keymap() or {}).get("layers", {}).get("vim", [])
        token = chars.casefold()
        return any(key.get("type") != "trans" and
                   str(key.get("tap", "")).casefold() == token for key in keys)

    def layer_entry(self, code, drawer):
        """Enter a one-shot layer and remember the exact stack to restore afterward."""
        restore = self.current_layers()
        if restore is None:
            restore = []
        preferred = {"alpha2": {"ALPHA2"}, "shifted2": {"SHIFTED ALPHA2"}}.get(drawer, set())
        ids = self.stack_with(restore, {drawer}, preferred)
        if ids == restore:
            return []
        self.layer_restore[code] = list(restore)
        return [{"kind": "layers", "ids": ids}]

    def hold_layers(self, code, chars=""):
        """Apply the physical layer needed for a literal key, preserving its stack."""
        if code in NAV_CODES:
            drawers, preferred = {"nav"}, {"NAVIGATION"}
            activator_idx = None
        elif chars in NUMBER_LAYER_CHARS:
            drawers, preferred = NUMBER_DRAWERS, {"NUMBERS"}
            activator_idx = NUMBERS_THUMB_IDX
        elif chars in SYMBOL_LAYER_CHARS:
            drawers, preferred = SYMBOL_DRAWERS, {"SYMBOLS"}
            activator_idx = SYMBOLS_THUMB_IDX
        else:
            return []
        restore = self.current_layers()
        if restore is None:
            restore = []
        ids = self.stack_with(restore, drawers, preferred)
        if ids == restore:
            return []
        self.layer_restore[code] = list(restore)
        if activator_idx is not None:
            self.layer_activator[code] = self.position_for_idx(activator_idx)
        return [{"kind": "layers", "ids": ids}]

    def restore_layers(self, code=None):
        """Restore the stack captured before this key's emulated layer hold."""
        ids = self.layer_restore.pop(code, None) if code is not None else None
        if ids is None:
            ids = self.current_layers()
        # An unknown state still needs an explicit base-layer restore. Omitting it is
        # what made NUMBERS stick after a missed heartbeat in the previous take.
        return [{"kind": "layers", "ids": list(ids or [])}]

    def messages(self, code, value):
        """Translate one evdev event, including the Diamond owner's layer gestures."""
        if code in MODS:
            return [self.message(code, value)]
        msg = self.message(code, value)
        if value == 2:
            return [msg]

        if value == 0:
            if code in self.layer_restore:
                return [msg, *self.restore_layers(code)]
            return [msg]

        chars = msg["chars"]
        prefix = self.hold_layers(code, chars)
        if not chars:
            self.previous_char = None
            return prefix + [msg]

        drawer = self.alpha2_drawer(chars)
        if drawer:
            entered = self.layer_entry(code, drawer)
            if not entered:
                # A real alpha2 stack is already active; do not manufacture a restore.
                self.remember_char(chars)
                return prefix + [msg]
            self.remember_char(chars)
            out = prefix + entered + [msg]
            # The layer's activator is the alpha2 thumb. Flash it after the character
            # resolves so the position freshness guard still lets the alpha2 key light.
            thumb = self.position_for_idx(ALPHA2_THUMB_IDX)
            if thumb is not None:
                out.extend([{"kind": "press", "pos": thumb},
                            {"kind": "release", "pos": thumb}])
            if chars.isalpha() and chars.isupper():
                shift_thumb = self.position_for_idx(SHIFT_THUMB_IDX)
                if shift_thumb is not None:
                    out.extend([{"kind": "press", "pos": shift_thumb},
                                {"kind": "release", "pos": shift_thumb}])
            return out

        out = prefix + [msg]
        activator = self.layer_activator.pop(code, None)
        if activator is not None:
            # Flash after the character so the HUD's fresh-position guard does not
            # hide the key on its Numbers or Symbols drawer.
            out.extend([{"kind": "press", "pos": activator},
                        {"kind": "release", "pos": activator}])
        if chars.isalpha() and chars.isupper():
            shift_thumb = self.position_for_idx(SHIFT_THUMB_IDX)
            if shift_thumb is not None:
                # Sticky shift is a modifier, not a shifted1 layer report. The key
                # itself is kept in the typed stream; this position flash shows the tap.
                out.extend([{"kind": "press", "pos": shift_thumb},
                            {"kind": "release", "pos": shift_thumb}])
        self.remember_char(chars)
        return out

    def find(self):
        for path in evdev.list_devices():
            try:
                dev = evdev.InputDevice(path)
            except PermissionError:
                self.log(f"rehearsal-feed: no permission for {path} (join the `input` group)")
                continue
            if self.match.lower() in dev.name.lower():
                return dev
            dev.close()
        return None

    async def run(self):
        """Wait for the injected device (ydotoold may start after us), then read it forever."""
        announced = False
        while True:
            dev = self.find()
            if dev is None:
                if not announced:
                    self.log(f"rehearsal-feed: no input device matching {self.match!r} yet; "
                             "start ydotoold (the rehearsal does) -- injected keys stay invisible")
                    announced = True
                await asyncio.sleep(2)
                continue
            announced = False
            self.log(f"rehearsal-feed: injected keys from {dev.path} ({dev.name})")
            try:
                async for ev in dev.async_read_loop():
                    if ev.type == E.EV_KEY:
                        messages = self.messages(ev.code, ev.value)
                        shift_pos = self.position_for_idx(SHIFT_THUMB_IDX)
                        for i, msg in enumerate(messages):
                            self.emit(msg)
                            # Alpha2 plus sticky shift are two taps, not a thumb combo. Let
                            # the HUD's combo window close before flashing the second thumb.
                            if (msg.get("kind") == "release" and i + 1 < len(messages) and
                                    messages[i + 1].get("kind") == "press" and
                                    messages[i + 1].get("pos") == shift_pos):
                                await asyncio.sleep(0.12)
            except OSError as e:
                self.log(f"rehearsal-feed: {dev.path} gone ({e})")
            finally:
                dev.close()
                self.held = {k: 0 for k in self.held}
                self.layer_restore.clear()
                self.layer_activator.clear()
                self.previous_char = None


def parse_args(argv=None):
    p = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    p.add_argument("--port", type=int, default=PORT, help=f"WebSocket port (default {PORT})")
    p.add_argument("--device", default=DEVICE,
                   help=f"name substring of the injected device (default {DEVICE!r})")
    p.add_argument("--no-keyboard", action="store_true",
                   help="skip the keyboard reader: no layers, the pages infer them (extras:)")
    p.add_argument("--config", help="zmk-layer-hud config (default: its own default)")
    p.add_argument("--debug", action="store_true", help="log layer messages to stderr")
    return p.parse_args(argv)


async def main(args):
    hub = hudfeed.Hub(debug=args.debug)
    loop = asyncio.get_running_loop()

    def emit(msg):
        loop.call_soon_threadsafe(lambda: asyncio.ensure_future(hub.send(msg)))

    feed = None
    if args.no_keyboard:
        hub.log("rehearsal-feed: no keyboard reader; the pages infer layers from the keys")
    else:
        # The real thing: keymap (with its live reload), device name, and the layers the
        # keyboard is actually on -- the daemon moves those, so the banner stays ground truth.
        feed = hudfeed.Feed(emit, log=hub.log, config=args.config).start()

    def true_layers():
        # The live layer stack sits on each reader stream's decoder (readers themselves expose
        # none); first stream that ever reported wins, None while none has. Reading is exact, so
        # a restore can never stick a wrong picture.
        reader = getattr(feed, "reader", None)
        for stream in getattr(reader, "_streams", None) or ():
            ids = getattr(getattr(stream, "decoder", None), "layers", None)
            if ids is not None:
                return list(ids)
        return None

    def cached_layers():
        # Hub.cache is updated for every message that reaches the pages. It is the stable fallback
        # between reader heartbeats; the direct decoder is only used before the first layer report.
        cached = hub.cache.get("layers")
        return list(cached.get("ids", [])) if cached is not None else true_layers()

    injected = InjectedKeys(emit, hub.log, args.device,
                            keymap=lambda: hub.cache.get("keymap"),
                            layers=cached_layers)
    asyncio.ensure_future(injected.run())

    try:
        import websockets
    except ImportError:
        sys.exit(f"python-websockets is required: .venv/bin/pip install websockets in {HUD}")
    async with websockets.serve(hub.handler, "127.0.0.1", args.port):
        hub.log(f"rehearsal-feed: ws://127.0.0.1:{args.port}")
        await asyncio.Event().wait()


if __name__ == "__main__":
    try:
        asyncio.run(main(parse_args()))
    except KeyboardInterrupt:
        pass
