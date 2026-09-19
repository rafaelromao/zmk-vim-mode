#!/usr/bin/env python3
"""The rehearsal's feed for zmk-layer-hud's pages: the keyboard's own signal channel, plus
the keys ydotool injects.

    python3 showcase/rehearsal-feed.py [--no-keyboard] [--device ydotool] [--debug]

A take needs nothing of this: zmk-layer-hud reads the Diamond's HID reports and draws exactly
what it did (`bash showcase/hud.sh`). A rehearsal types with ydotool, which writes to
/dev/uinput and never reaches the keyboard, so those keys are invisible to that feed. This one
runs zmk-layer-hud's own reader -- the keymap and its live reload, the device name, and the
layers the keyboard really is on, which follow `zmk-vim-mode set …` because the daemon writes
the mode to the keyboard -- and adds the injected keys read off ydotool's virtual device. The
pages then light them against the real layer stack, exactly as they light real typing.
Arrow keys and digits are drawn as held layers (the nav/numbers drawers), never
as combos — the way the takes type them. The release puts the keyboard's own
layers back at once (a same-valued heartbeat re-asserts nothing, so the
restore is the correction, not a speedup).

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
# Keys a human never combos: arrows and digits come off held layers, so the
# rehearsal holds them too — the banner reads the layer, exactly as on camera.
NAV_CODES = {E.KEY_UP, E.KEY_DOWN, E.KEY_LEFT, E.KEY_RIGHT,
             E.KEY_HOME, E.KEY_END, E.KEY_PAGEUP, E.KEY_PAGEDOWN}
DIGIT_CODES = {E.KEY_0, E.KEY_1, E.KEY_2, E.KEY_3, E.KEY_4,
               E.KEY_5, E.KEY_6, E.KEY_7, E.KEY_8, E.KEY_9}
NAV_DRAWERS = {"nav"}
DIGIT_DRAWERS = {"numbers"}


class InjectedKeys:
    """Reads one evdev device and emits zmk-layer-hud key messages for what it types."""

    def __init__(self, emit, log, match=DEVICE, keymap=None, true_layers=None):
        self.emit, self.log, self.match = emit, log, match
        self.held = {"cmd": 0, "ctrl": 0, "alt": 0, "shift": 0}
        # The keymap message (for drawer -> ZMK layer ids) and the keyboard's
        # own current layer ids (to put back after an emulated hold). Both are
        # best effort: without them the keys still light, just with no banner.
        self.keymap = keymap or (lambda: None)
        self.true_layers = true_layers or (lambda: None)

    def flags(self):
        return {k: v > 0 for k, v in self.held.items()} | {"fn": False}

    def message(self, code, value):
        """One evdev event -> a key message (value: 1 down, 0 up, 2 repeat)."""
        if code in MODS:
            name = MODS[code]
            self.held[name] = max(0, self.held[name] + (1 if value == 1 else -1 if value == 0 else 0))
            return {"kind": "key", "type": "flagsChanged", "name": "", "chars": "", "code": 0,
                    "flags": self.flags(), "repeat": False}
        flags = self.flags()
        chars = CONTROL_CHARS.get(code, "")
        if not chars and code in CHARS:
            chars = CHARS[code][1 if flags["shift"] else 0]
        name = NAMED.get(code) or (chars if chars and chars != " " else
                                   E.KEY.get(code, str(code)).replace("KEY_", "").lower())
        return {"kind": "key", "type": "keyDown" if value else "keyUp", "name": name,
                "chars": chars, "code": code, "flags": flags, "repeat": value == 2}

    def drawer_ids(self, drawers):
        """ZMK layer ids drawn with one of these drawers, from the keymap message."""
        km = self.keymap() or {}
        ids = set()
        for lid, info in (km.get("zmk_layers") or {}).items():
            if isinstance(info, dict) and info.get("drawer") in drawers:
                try:
                    ids.add(int(info.get("id", lid)))
                except (TypeError, ValueError):
                    pass
        return sorted(ids)

    def hold_layers(self, code):
        """The layers message a held layer would have produced for this key."""
        if code in NAV_CODES:
            ids = self.drawer_ids(NAV_DRAWERS)
        elif code in DIGIT_CODES and self.held.get("shift", 0) == 0:
            ids = self.drawer_ids(DIGIT_DRAWERS)
        else:
            return []
        return [{"kind": "layers", "ids": ids}] if ids else []

    def restore_layers(self):
        """The keyboard's own layers again, after an emulated hold. [] is a
        real state (base layer only), not a missing one — only None skips."""
        ids = self.true_layers()
        return [{"kind": "layers", "ids": list(ids)}] if ids is not None else []

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
                             "start ydotoold (the rehearsal does) — injected keys stay invisible")
                    announced = True
                await asyncio.sleep(2)
                continue
            announced = False
            self.log(f"rehearsal-feed: injected keys from {dev.path} ({dev.name})")
            try:
                async for ev in dev.async_read_loop():
                    if ev.type == E.EV_KEY:
                        if ev.value == 1:
                            for m in self.hold_layers(ev.code):
                                self.emit(m)
                        self.emit(self.message(ev.code, ev.value))
                        if ev.value == 0 and (ev.code in NAV_CODES or ev.code in DIGIT_CODES):
                            for m in self.restore_layers():
                                self.emit(m)
            except OSError as e:
                self.log(f"rehearsal-feed: {dev.path} gone ({e})")
            finally:
                dev.close()
                self.held = {k: 0 for k in self.held}


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
        # keyboard is actually on — the daemon moves those, so the banner stays ground truth.
        feed = hudfeed.Feed(emit, log=hub.log, config=args.config).start()

    def true_layers():
        # The live layer stack sits on each reader stream's decoder (readers
        # themselves expose none); first stream that ever reported wins, None
        # while none has. Reading is exact, so a restore can never stick a
        # wrong picture (a same-valued heartbeat re-asserts nothing).
        reader = getattr(feed, "reader", None)
        for stream in getattr(reader, "_streams", None) or ():
            ids = getattr(getattr(stream, "decoder", None), "layers", None)
            if ids is not None:
                return list(ids)
        return None

    injected = InjectedKeys(emit, hub.log, args.device,
                            keymap=lambda: hub.cache.get("keymap"),
                            true_layers=true_layers)
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
