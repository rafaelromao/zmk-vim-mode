#!/usr/bin/env python3
"""Linux host for the showcase HUD pages: key events + daemon decisions over a WebSocket.

The HUD (hud/index.html) and the typed-keys strip (hud/keys.html) are plain web pages; on
macOS Hammerspoon drives them, on Linux this script does. Open the pages with
`?ws=ws://127.0.0.1:8766` (see hud.sh) and they connect here.

Messages sent to every client (one JSON object per message):
  {"kind":"key","type":"keyDown","name":"space","chars":" ","code":57,
   "flags":{"cmd":false,"ctrl":false,"alt":false,"shift":false,"fn":false},"repeat":false}
  {"kind":"mode","code":1,"mode":"normal","reason":"nvim client"}
A client sending {"kind":"close"} (the ✕ button) makes this script exit.

Inputs:
  * evdev  every keyboard under /dev/input (needs read access: the daemon's udev rule gives
           uaccess for the ZMK keyboard, `input` group membership covers the rest)
  * journalctl --user -u zmk-vim-mode -f   the daemon's `msg=decision …` lines (systemd unit),
           falling back to polling `zmk-vim-mode status --json` when journalctl is missing

Dependencies (Arch): python-evdev python-websockets.
UNTESTED on real hardware as of the handoff: written on macOS, ported from hud/hud.lua.
"""

import asyncio
import json
import os
import re
import shutil
import subprocess
import sys

try:
    import evdev
    from evdev import ecodes as E
except ImportError:
    sys.exit("python-evdev is required: sudo pacman -S python-evdev")
try:
    import websockets
except ImportError:
    sys.exit("python-websockets is required: sudo pacman -S python-websockets")

HOST, PORT = "127.0.0.1", int(os.environ.get("ZMKHUD_PORT", "8766"))
BINARY = os.path.expanduser("~/.local/bin/zmk-vim-mode")

# US layout: evdev key -> (char, shifted char). The HUD resolves characters against the
# keymap-drawer legends, so this only needs to match what the host layout produces.
CHARS = {
    **{getattr(E, f"KEY_{c}"): (c.lower(), c) for c in "ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
    E.KEY_1: ("1", "!"), E.KEY_2: ("2", "@"), E.KEY_3: ("3", "#"), E.KEY_4: ("4", "$"),
    E.KEY_5: ("5", "%"), E.KEY_6: ("6", "^"), E.KEY_7: ("7", "&"), E.KEY_8: ("8", "*"),
    E.KEY_9: ("9", "("), E.KEY_0: ("0", ")"), E.KEY_MINUS: ("-", "_"), E.KEY_EQUAL: ("=", "+"),
    E.KEY_LEFTBRACE: ("[", "{"), E.KEY_RIGHTBRACE: ("]", "}"), E.KEY_BACKSLASH: ("\\", "|"),
    E.KEY_SEMICOLON: (";", ":"), E.KEY_APOSTROPHE: ("'", '"'), E.KEY_GRAVE: ("`", "~"),
    E.KEY_COMMA: (",", "<"), E.KEY_DOT: (".", ">"), E.KEY_SLASH: ("/", "?"), E.KEY_SPACE: (" ", " "),
}
# Named keys, spelled the way hs.keycodes.map spells them (the pages share one table).
NAMED = {
    E.KEY_SPACE: "space", E.KEY_ENTER: "return", E.KEY_ESC: "escape", E.KEY_BACKSPACE: "delete",
    E.KEY_DELETE: "forwarddelete", E.KEY_TAB: "tab", E.KEY_LEFT: "left", E.KEY_RIGHT: "right",
    E.KEY_UP: "up", E.KEY_DOWN: "down", E.KEY_HOME: "home", E.KEY_END: "end",
    E.KEY_PAGEUP: "pageup", E.KEY_PAGEDOWN: "pagedown",
    **{getattr(E, f"KEY_F{n}"): f"f{n}" for n in range(1, 13)},
}
MODS = {
    E.KEY_LEFTSHIFT: "shift", E.KEY_RIGHTSHIFT: "shift", E.KEY_LEFTCTRL: "ctrl", E.KEY_RIGHTCTRL: "ctrl",
    E.KEY_LEFTALT: "alt", E.KEY_RIGHTALT: "alt", E.KEY_LEFTMETA: "cmd", E.KEY_RIGHTMETA: "cmd",
}

clients = set()
last_mode = None
held = {"cmd": 0, "ctrl": 0, "alt": 0, "shift": 0}


async def broadcast(msg):
    if not clients:
        return
    data = json.dumps(msg, ensure_ascii=False)
    await asyncio.gather(*(c.send(data) for c in list(clients)), return_exceptions=True)


def flags():
    return {k: v > 0 for k, v in held.items()} | {"fn": False}


def key_event(code, value):
    """One evdev key event -> HUD message (or None). value: 1 down, 0 up, 2 repeat."""
    if code in MODS:
        held[MODS[code]] = max(0, held[MODS[code]] + (1 if value == 1 else -1 if value == 0 else 0))
        return {"kind": "key", "type": "flagsChanged", "name": MODS[code], "chars": "", "code": code,
                "flags": flags(), "repeat": False}
    if value == 0:
        typ = "keyUp"
    else:
        typ = "keyDown"
    chars = ""
    if code in CHARS and code not in (E.KEY_SPACE,):
        chars = CHARS[code][1 if held["shift"] else 0]
    elif code == E.KEY_SPACE:
        chars = " "
    elif code == E.KEY_ENTER:
        chars = "\r"
    elif code == E.KEY_ESC:
        chars = "\x1b"
    name = NAMED.get(code) or (chars if chars and chars != " " else E.KEY.get(code, str(code)).replace("KEY_", "").lower())
    return {"kind": "key", "type": typ, "name": name, "chars": chars, "code": code,
            "flags": flags(), "repeat": value == 2}


def keyboards():
    devs = []
    for path in evdev.list_devices():
        try:
            d = evdev.InputDevice(path)
        except PermissionError:
            print(f"keyfeed: no permission for {path} (add yourself to the input group or fix udev)", file=sys.stderr)
            continue
        caps = d.capabilities().get(E.EV_KEY, [])
        if E.KEY_A in caps and E.KEY_Z in caps:
            devs.append(d)
    return devs


async def read_keyboard(dev):
    print(f"keyfeed: reading {dev.path} ({dev.name})")
    try:
        async for ev in dev.async_read_loop():
            if ev.type == E.EV_KEY:
                msg = key_event(ev.code, ev.value)
                if msg:
                    await broadcast(msg)
    except OSError as e:
        print(f"keyfeed: {dev.path} gone ({e})")


DECISION = re.compile(r'msg=decision mode=(\S+) code=(\d) reason="([^"]*)"')


async def apply_line(line):
    global last_mode
    m = DECISION.search(line)
    if not m:
        return
    msg = {"kind": "mode", "code": int(m.group(2)), "mode": m.group(1), "reason": m.group(3)}
    if msg != last_mode:
        last_mode = msg
        await broadcast(msg)


async def follow_journal():
    if not shutil.which("journalctl"):
        return await poll_status()
    proc = await asyncio.create_subprocess_exec(
        "journalctl", "--user", "-u", "zmk-vim-mode", "-f", "-o", "cat", "-n", "50",
        stdout=asyncio.subprocess.PIPE, stderr=asyncio.subprocess.DEVNULL)
    print("keyfeed: following journalctl --user -u zmk-vim-mode")
    async for raw in proc.stdout:
        await apply_line(raw.decode("utf-8", "replace"))
    print("keyfeed: journalctl ended; polling status instead")
    await poll_status()


async def poll_status():
    while True:
        try:
            out = subprocess.run([BINARY, "status", "--json"], capture_output=True, text=True, timeout=2).stdout
            st = json.loads(out)
            await apply_line(f'msg=decision mode={st["mode"]} code={st["code"]} reason="{st.get("reason", "")}"')
        except Exception:
            pass
        await asyncio.sleep(0.2)


async def handler(ws):
    clients.add(ws)
    try:
        if last_mode:
            await ws.send(json.dumps(last_mode, ensure_ascii=False))
        async for raw in ws:
            try:
                if json.loads(raw).get("kind") == "close":
                    print("keyfeed: close requested by the page")
                    os._exit(0)
            except json.JSONDecodeError:
                pass
    finally:
        clients.discard(ws)


async def main():
    devs = keyboards()
    if not devs:
        print("keyfeed: no readable keyboard under /dev/input", file=sys.stderr)
    async with websockets.serve(handler, HOST, PORT):
        print(f"keyfeed: ws://{HOST}:{PORT}")
        await asyncio.gather(follow_journal(), *(read_keyboard(d) for d in devs))


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass
