#!/usr/bin/env python3
"""Rehearsal runner (Omarchy / Hyprland): performs the scripted actions of SCRIPT.md with
synthesized keystrokes (ydotool/uinput), asks the daemon what it decided after each one, and
writes a PASS/FAIL report to showcase/run/rehearsal.log.

    python3 showcase/rehearse.py [3|4|5|6|7|8|all] [--verbose]

Run showcase/prepare.sh first. Keep your hands off the keyboard while it runs.
Needs: ydotool (+ ydotoold, user in the `input` group), hyprctl, the daemon, Ghostty, editors.

The HUD: synthesized keys never reach the Diamond, and zmk-layer-hud draws only what the
keyboard reports, so a rehearsal runs its own copy of it — rehearsal-panel.py with
rehearsal-feed.py, on their own port — which adds the injected keys to the layers the keyboard
really is on. See rehearsal-feed.py. Takes use `bash showcase/hud.sh` instead.

Notes:
  * focus is by Hyprland window class and verified by address before typing. Demo Ghostty
    uses its standard class (recognized by the daemon) and is selected by process ID.
  * the demo shell is Bash with env/demo.bashrc.
  * the chords are the Linux defaults (see CHORDS below): VS Code Ctrl+P / Ctrl+Shift+E /
    Ctrl+`, IntelliJ Ctrl+Shift+N (go to file) / Alt+F12, Obsidian Ctrl+E / Ctrl+Shift+F.
    The Meh chord (Ctrl+Alt+Shift+B) comes off the keymap and is the same everywhere.
  * the daemon reads window titles and the accessibility bus (--atspi), so VS Code's tool
    windows report `tool window focused: …` reasons.
"""

import json
import os
import re
import signal
import subprocess
import sys
import time

SHOW = os.path.dirname(os.path.abspath(__file__))
RUN = os.path.join(SHOW, "run")
BINARY = os.path.expanduser("~/.local/bin/zmk-vim-mode")
VAULT = "Demo"
CHAR_GAP = 0.16
VERBOSE = "--verbose" in sys.argv or os.environ.get("VERBOSE") == "1"

os.makedirs(RUN, exist_ok=True)
report = open(os.path.join(RUN, "rehearsal.log"), "w")
results = {"pass": 0, "fail": 0}
target_address = None
DEMO_CLASS = "com.mitchellh.ghostty"
version = json.loads(subprocess.check_output(["hyprctl", "version", "-j"], text=True))
LUA = tuple(int(n) for n in version["version"].split(".")[:2]) >= (0, 55)


def log(line):
    stamp = time.strftime("%H:%M:%S ")
    print(stamp + line, flush=True)
    report.write(stamp + line + "\n")
    report.flush()


def status():
    try:
        out = subprocess.run([BINARY, "status", "--json"], capture_output=True, text=True, timeout=2).stdout
        return json.loads(out)
    except Exception as e:  # noqa: BLE001
        return {"mode": "?", "reason": f"status failed: {e}"}


def sh(cmd, wait=0.6):
    if VERBOSE:
        log("  · $ " + cmd)
    subprocess.run(cmd, shell=True)
    time.sleep(wait)


def ensure_ydotoold():
    """ydotool injects via /dev/uinput (kernel-level, identical to a real keyboard).
    wtype's Wayland virtual-keyboard events are silently dropped by VS Code's input
    layer, so the rehearsal types everything through ydotool. Needs `input` group."""
    if subprocess.run(["pgrep", "-x", "ydotoold"], capture_output=True).returncode != 0:
        subprocess.Popen(["ydotoold"], stdout=subprocess.DEVNULL,
                         stderr=subprocess.DEVNULL, start_new_session=True)
        time.sleep(1)
    subprocess.run(["ydotool", "key", "119:1", "119:0"],
                   capture_output=True)  # no-op probe: Break key, harmless anywhere


PANEL = os.path.join(SHOW, "rehearsal-panel.py")


def start_hud():
    """The rehearsal's own copy of the layer HUD (rehearsal-panel.py): zmk-layer-hud's pages fed
    with the keys ydotool injects on top of the layers the keyboard really is on. A take uses
    `bash showcase/hud.sh` instead, which reads the keyboard alone. NO_HUD=1 skips this: the
    daemon expectations below do not depend on it."""
    if os.environ.get("NO_HUD") == "1":
        log("HUD skipped (NO_HUD=1)")
        return None
    # A take's HUD anchors to the same corner and reserves the same rail: two of them stack.
    if subprocess.run(["pgrep", "-f", "zmk-layer-hud/host/linux/panel.py"],
                      capture_output=True).returncode == 0:
        sys.exit("The take's HUD is running and would sit under the rehearsal's copy: "
                 "`bash showcase/hud.sh stop` first (or rehearse with NO_HUD=1).")
    subprocess.run(["pkill", "-f", PANEL], capture_output=True)
    time.sleep(0.5)
    output = open(os.path.join(RUN, "rehearsal-panel.log"), "w")
    proc = subprocess.Popen([sys.executable, "-u", PANEL], stdout=output, stderr=output,
                            start_new_session=True)
    time.sleep(2)
    if proc.poll() is not None:
        sys.exit("The rehearsal HUD did not start; see showcase/run/rehearsal-panel.log "
                 "(and run/rehearsal-feed.log). Rehearse without it with NO_HUD=1.")
    log("HUD started (rehearsal-panel.py)")
    return proc


def stop_hud(proc):
    if proc is None:
        return
    try:
        os.killpg(proc.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass


# Linux evdev keycodes for the non-printable keys the segments need.
KEYCODES = {
    "Escape": 1, "Return": 28, "Tab": 15, "space": 57, "BackSpace": 14,
    "grave": 41, "backslash": 43,
    "F1": 59, "F12": 88,
    "Up": 103, "Down": 108, "Left": 105, "Right": 106,
    "Home": 102, "End": 107, "PageUp": 104, "PageDown": 109,
    "ctrl": 29, "shift": 42, "alt": 56, "super": 125,
    "b": 48, "e": 18, "f": 33, "n": 49, "p": 25,
}
MODS = {"ctrl", "shift", "alt", "super"}


def send(*args):
    if target_address:
        active = json.loads(subprocess.check_output(["hyprctl", "activewindow", "-j"], text=True))
        if active.get("address") != target_address:
            raise RuntimeError(f"Focus left rehearsal window; refusing to type into {active.get('title')!r}")
    subprocess.run(["ydotool", *args], check=True)


def keys(text, wait=0.6):
    """Type text; "\\n" is a Return key. Plain runs go out as one `ydotool type`
    call, at the pace the script asks for on camera; rehearsal-feed.py reads them off
    ydotool's virtual device so the HUD shows them like Diamond keystrokes."""
    run = ""
    def flush():
        nonlocal run
        if run:
            if VERBOSE:
                log("  · type " + run)
            send("type", run)
            time.sleep(CHAR_GAP)
            run = ""
    for ch in text:
        if ch == "\n":
            flush()
            if VERBOSE:
                log("  · type ⏎")
            send("key", "28:1", "28:0")
            time.sleep(0.35)
        else:
            run += ch
    flush()
    time.sleep(wait)


def key(mods, k, wait=0.6):
    """A chord: mods is a list of modifier names (ctrl, alt, shift, super);
    k is a KEYCODES name (Return, Escape, F1, Down, grave, ...) or a letter."""
    if VERBOSE:
        log("  · " + "+".join(mods + [k]))
    if k in MODS:
        raise ValueError(f"{k} is a modifier, not a key")
    code = KEYCODES.get(k)
    if code is None:
        if len(k) == 1 and k.isalpha():
            code = KEYCODES[k.lower()]
        else:
            raise ValueError(f"no keycode for {k!r}")
    args = [f"{KEYCODES[m]}:1" for m in mods] + [f"{code}:1", f"{code}:0"]
    args += [f"{KEYCODES[m]}:0" for m in reversed(mods)]
    send("key", *args)
    time.sleep(wait)


def esc(wait=0.6):
    key([], "Escape", wait)


def palette(command, wait=2.0):
    """Run a VS Code command via the F1 palette. Ctrl+` never reaches this box's
    VS Code (chord swallowed with zero effect, fresh-instance verified)."""
    key([], "F1", 1.2)
    keys(command, 0.8)
    key([], "Return", wait)


def focus(cls, wait=2.0, pid=None):
    global target_address
    if VERBOSE:
        log("  · focus " + cls)
    windows = json.loads(subprocess.check_output(["hyprctl", "clients", "-j"], text=True))
    matches = [w for w in windows if re.fullmatch(cls, w["class"]) and (pid is None or w["pid"] == pid)]
    # IDEs can leave an untitled transient frame (splash/welcome) beside the project
    # window; prefer windows that already have a title.
    titled = [w for w in matches if w["title"]]
    if len(titled) == 1:
        matches = titled
    if len(matches) != 1:
        raise RuntimeError(f"Expected one {cls!r} window, found {len(matches)}; refusing to type")
    address = matches[0]["address"]
    if LUA:
        # Maximized fills the work area, which excludes the HUD's reserved right rail.
        subprocess.run(["hyprctl", "dispatch",
                        f'hl.dsp.window.fullscreen({{mode="maximized", action="set", window="address:{address}"}})'],
                       capture_output=True, check=True)
    args = [f'hl.dsp.focus({{window="address:{address}"}})'] if LUA else ["focuswindow", f"address:{address}"]
    subprocess.run(["hyprctl", "dispatch", *args], capture_output=True, check=True)
    time.sleep(wait)
    active = json.loads(subprocess.check_output(["hyprctl", "activewindow", "-j"], text=True))
    if active.get("address") != address:
        raise RuntimeError(f"Could not focus {cls!r}; refusing to type")
    target_address = address


def wait_window(cls, timeout=90.0, pid=None):
    """Poll until exactly one titled window matches (launches are slow; transient
    doubles while an old window closes settle on their own)."""
    end = time.time() + timeout
    while time.time() < end:
        windows = json.loads(subprocess.check_output(["hyprctl", "clients", "-j"], text=True))
        matches = [w for w in windows if re.fullmatch(cls, w["class"]) and (pid is None or w["pid"] == pid)]
        titled = [w for w in matches if w["title"]]
        if len(titled) == 1:
            return titled[0]
        time.sleep(0.5)
    raise RuntimeError(f"no single {cls!r} window appeared; refusing to type")


def place(address, workspace):
    """Move a window to its demo workspace and maximize it beside the HUD's right rail."""
    subprocess.run(["hyprctl", "dispatch",
                    f'hl.dsp.window.move({{workspace="{workspace}", window="address:{address}"}})'],
                   capture_output=True, check=True)
    subprocess.run(["hyprctl", "dispatch",
                    f'hl.dsp.window.fullscreen({{mode="maximized", action="set", window="address:{address}"}})'],
                   capture_output=True, check=True)


def wait_editor(app, timeout=40.0):
    """Ground-truth gate before scripted typing: the daemon must see this app's
    editor focused, not a dialog, sidebar, picker or splash. Window focus alone
    cannot see those; mode expects after the fact can only fail, not prevent."""
    end = time.time() + timeout
    while time.time() < end:
        st = status()
        front = (st.get("frontmost") or {}).get("class", "")
        widget_editor = (st.get("widget") or {}).get("editor", False)
        clients = st.get("clients") or []
        if app == "vscode":
            ok = front == "code" and widget_editor and any(
                c.get("app") == "vscode" and c.get("focused") for c in clients)
        elif app == "intellij":
            ok = front in ("jetbrains-idea", "jetbrains-idea-ultimate", "jetbrains-idea-community")
        elif app == "obsidian":
            ok = front in ("obsidian", "md.obsidian.Obsidian")
        else:
            ok = False
        if ok:
            log(f"  · editor focus confirmed: {app}")
            return True
        time.sleep(0.5)
    raise RuntimeError(f"{app} editor never reported focus; refusing to type")


def expect(mode, reason=None):
    st = status()
    ok_mode = st.get("mode") == mode
    ok_reason = reason is None or reason in str(st.get("reason", ""))
    ok = ok_mode and ok_reason
    verdict = "PASS" if ok else "FAIL"
    results["pass" if ok else "fail"] += 1
    want = f"mode={mode}" + (f" reason~{reason}" if reason else "")
    log(f'{verdict}  want {want}  got mode={st.get("mode")} code={st.get("code")} reason="{st.get("reason")}"')
    if not ok:
        raise RuntimeError("Rehearsal stopped at the first unexpected state; see rehearsal.log")


def open_demo_ghostty():
    if LUA:
        subprocess.run(["hyprctl", "dispatch", 'hl.dsp.focus({workspace="8"})'], check=True)
    conf = os.path.join(SHOW, "env", "ghostty-demo.conf")
    with open(os.path.join(RUN, "ghostty.log"), "w") as output:
        proc = subprocess.Popen(["ghostty", "--gtk-single-instance=false", f"--config-file={conf}",
                                 f"--working-directory={os.path.join(SHOW, 'demo-go')}", f"--class={DEMO_CLASS}",
                                 "-e", "/bin/bash", "--noprofile", "--rcfile",
                                 os.path.join(SHOW, "env", "demo.bashrc"), "-i"],
                                stdout=output, stderr=output)
    time.sleep(3)
    focus(re.escape(DEMO_CLASS), 1.5, pid=proc.pid)


def vim_tour():
    for k in "jjjklllhh":
        keys(k, 0.35)
    expect("normal")
    for k in ["w", "w", "e", "b", "b", "0", "$", "0"]:
        keys(k, 0.35)
    keys("i", 0.5); expect("insert")
    esc(0.5); expect("normal")
    keys("a", 0.5); expect("insert")
    esc(0.5); expect("normal")
    keys("gg", 0.5)


def seg3():
    log("--- segment 3: how it works (set overrides)")
    sh(f"{BINARY} set insert", 0.5); expect("insert")
    sh(f"{BINARY} set normal", 0.5); expect("normal")
    sh(f"{BINARY} set off", 0.5); expect("off")
    sh(f"{BINARY} set off", 0.5)
    st = status()
    ok = st.get("override") is None
    results["pass" if ok else "fail"] += 1
    log(("PASS" if ok else "FAIL") + "  override cleared")


def seg4():
    log("--- segment 4: Neovim")
    open_demo_ghostty()
    keys("nvim\n", 4); expect("raw", "nvim client")
    keys("f", 1.5); keys("modes.go", 1.0); key([], "Return", 1.5)
    expect("normal", "nvim client")
    vim_tour()
    keys("/Compose\n", 1.0); expect("normal")
    keys("A", 0.5); expect("insert")
    keys(" // bit 0 of the code", 0.5)
    esc(0.5); expect("normal")
    keys("u", 0.4)
    keys("v", 0.5); expect("visual")
    keys("jj", 0.3); keys("y", 0.5); expect("normal")
    keys(":", 0.5); expect("cmdline")
    esc(0.5); expect("normal")
    # keys() adds CHAR_GAP even after the last character; stay below timeoutlen.
    keys(" ", 0.05); expect("raw", "nvim client")
    esc(0.5); expect("normal")
    keys(" ff", 1.2); expect("insert")
    keys("readme", 0.8); expect("insert")
    esc(0.4); esc(0.6); expect("normal")
    keys(" e", 1.2); expect("raw")
    keys("j", 0.4); keys("j", 0.4); keys("k", 0.5); expect("raw")
    keys(" e", 1.0); expect("normal")
    keys(":terminal\n", 1.2); keys("i", 0.6); expect("raw")
    keys("go test ./...\n", 2.5); expect("raw")
    key(["ctrl"], "backslash", 0.2); key(["ctrl"], "n", 0.6); expect("normal")
    keys(":bd!\n", 0.8)
    keys(" l", 1.2); expect("raw")
    keys("q", 0.8); expect("normal")
    keys(":qa!\n", 2.0)
    keys("exit\n", 0.8)


def seg5():
    log("--- segment 5: VS Code")
    windows = json.loads(subprocess.check_output(["hyprctl", "clients", "-j"], text=True))
    if not any(w["class"] == "code" for w in windows):
        sh(f"code '{SHOW}/demo-go.code-workspace'", 5)
    place(wait_window("code")["address"], 5)
    focus("code", 1)
    key(["ctrl"], "p", 0.8); keys("modes.go", 0.6); key([], "Return", 2.0)
    esc(0.8); expect("normal", "client")
    wait_editor("vscode")
    vim_tour()
    keys("/Kana\n", 1.0); expect("normal")
    keys("A", 0.5); expect("insert")
    keys(" // bit 1 of the code", 0.4)
    esc(0.5); expect("normal")
    keys("v", 0.5); expect("visual")
    esc(0.5); expect("normal")
    palette("View: Toggle Integrated Terminal"); expect("raw", "tool window")
    keys("go run ./cmd/vimmode\n", 2.5); expect("raw", "tool window")
    palette("View: Toggle Integrated Terminal"); expect("normal")
    key([], "F1", 1.2); expect("raw")
    keys("keyboard", 0.9); expect("raw")
    esc(0.8); expect("normal")
    key(["ctrl", "shift"], "e", 1.2); expect("raw", "tool window")
    key([], "Down", 0.4); key([], "Down", 0.5); expect("raw")
    key(["ctrl", "shift"], "e", 1.2); expect("normal")
    keys("u", 0.4)


def seg6():
    log("--- segment 6: IntelliJ IDEA")
    place(wait_window("^(jetbrains-idea|jetbrains-idea-ultimate|jetbrains-idea-community)$")["address"], 6)
    focus("^(jetbrains-idea|jetbrains-idea-ultimate|jetbrains-idea-community)$", 2)
    key(["ctrl", "shift"], "n", 1.0); keys("ModeTable", 0.8); key([], "Return", 2.0)
    expect("normal", "intellij")          # no Esc: IdeaVim beeps on Esc in normal mode
    wait_editor("intellij")
    vim_tour()
    keys("/COMPOSE\n", 1.0)
    keys("A", 0.5); expect("insert")
    keys(" // Compose is bit 0", 0.4)
    esc(0.5); expect("normal")
    keys("v", 0.5); expect("visual")
    esc(0.5); expect("normal")
    keys(":", 0.6); expect("raw", "intellij client raw")   # IdeaVim's ex line is a separate component
    esc(0.6); expect("normal")
    key(["ctrl", "alt", "shift"], "b", 2.0); expect("raw", "intellij")   # Meh+B: project tool window
    keys("readme", 0.9); expect("raw")
    esc(0.5); esc(1.0); expect("normal")
    key(["alt"], "F12", 1.5); expect("raw")
    keys("ls\n", 1.5); expect("raw")
    key(["alt"], "F12", 1.2); expect("normal")
    keys("u", 0.4)


def seg7():
    log(f"--- segment 7: Obsidian (vault {VAULT})")
    sh(f"xdg-open 'obsidian://open?vault={VAULT}&file=Tasks'", 3)
    place(wait_window("obsidian|md\\.obsidian\\.Obsidian")["address"], 7)
    focus("obsidian|md\\.obsidian\\.Obsidian", 1)
    esc(0.8); expect("normal", "obsidian")
    wait_editor("obsidian")
    vim_tour()
    keys("jj", 0.3); keys("A", 0.5); expect("insert")
    keys(" (rehearsal)", 0.4)
    esc(0.5); expect("normal")
    keys("u", 0.4)
    keys(":", 0.6); expect("cmdline")
    esc(0.6); expect("normal")
    key(["ctrl"], "e", 1.2); expect("raw")            # reading view
    key(["ctrl"], "e", 1.2); expect("normal")
    key(["ctrl", "shift"], "f", 1.2); expect("raw")   # search pane
    keys("layer", 1.0); expect("raw")
    esc(0.4)
    # back into the note: click the first task line (window-relative 60 %, 22 %)
    focus("obsidian|md\\.obsidian\\.Obsidian", 0.2)
    geo = json.loads(subprocess.run(["hyprctl", "activewindow", "-j"], capture_output=True, text=True).stdout)
    x = geo["at"][0] + geo["size"][0] * 0.6
    y = geo["at"][1] + geo["size"][1] * 0.22
    subprocess.run(["hyprctl", "dispatch", "movecursor", str(int(x)), str(int(y))], capture_output=True)
    time.sleep(0.2)
    # a left click: ydotool (needs ydotoold) or wlrctl, whichever the box has
    subprocess.run("ydotool click 0xC0 2>/dev/null || wlrctl pointer click left 2>/dev/null", shell=True)
    time.sleep(1.0); expect("normal")


def seg8():
    log("--- segment 8: everywhere else")
    open_demo_ghostty()
    expect("off", "terminal without nvim client")
    sh(f"{BINARY} set raw", 0.5); expect("raw")
    sh(f"{BINARY} set raw", 0.5)
    st = status()
    ok = st.get("override") is None
    results["pass" if ok else "fail"] += 1
    log(("PASS" if ok else "FAIL") + "  override cleared")
    expect("off")
    keys("exit\n", 0.8)


SEGMENTS = {"3": seg3, "4": seg4, "5": seg5, "6": seg6, "7": seg7, "8": seg8}

if __name__ == "__main__":
    ensure_ydotoold()
    hud = start_hud()
    which = next((a for a in sys.argv[1:] if not a.startswith("--")), "all")
    order = list(SEGMENTS) if which == "all" else [which]
    log(f"rehearsal {which} — hands off the keyboard")
    print("starting in 3 seconds…")
    time.sleep(3)
    try:
        for k in order:
            if k in SEGMENTS:
                time.sleep(1.5)
                SEGMENTS[k]()
            else:
                log(f"unknown segment {k}")
    finally:
        stop_hud(hud)
    log(f"DONE  pass={results['pass']} fail={results['fail']}")
    sys.exit(1 if results["fail"] else 0)
