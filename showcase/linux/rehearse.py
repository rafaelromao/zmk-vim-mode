#!/usr/bin/env python3
"""Linux (Omarchy / Hyprland) port of hud/rehearse.lua: performs the scripted actions of
SCRIPT.md with synthesized keystrokes (wtype), asks the daemon what it decided after each one,
and writes a PASS/FAIL report to showcase/run/rehearsal.log.

    python3 showcase/linux/rehearse.py [3|4|5|6|7|8|all] [--verbose]

Run showcase/linux/prepare.sh first. Keep your hands off the keyboard while it runs.
Needs: wtype (Wayland typing), hyprctl, the zmk-vim-mode daemon, Ghostty, the editors.

Port notes (UNTESTED as of the handoff, ported 1:1 from the Lua):
  * focus is by Hyprland window class and verified by address before typing. Demo Ghostty
    uses its standard class (recognized by the daemon) and is selected by process ID.
  * the Linux demo shell is Bash with env/demo.bashrc; macOS keeps its Zsh configuration.
  * "cmd" chords from the Mac become "super" or their Linux defaults (see CHORDS below):
    VS Code Ctrl+P / Ctrl+Shift+E / Ctrl+`, IntelliJ Ctrl+Shift+N (go to file) / Alt+F12,
    Obsidian Ctrl+E / Ctrl+Shift+F. The Meh chord (Ctrl+Alt+Shift+B) is the same everywhere.
  * the daemon on Linux reads window titles and the accessibility bus (--atspi), so VS Code's
    tool windows report the same `tool window focused: …` reasons as on macOS.
"""

import json
import os
import re
import subprocess
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
SHOW = os.path.abspath(os.path.join(HERE, ".."))
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


def wtype(*args):
    if target_address:
        active = json.loads(subprocess.check_output(["hyprctl", "activewindow", "-j"], text=True))
        if active.get("address") != target_address:
            raise RuntimeError(f"Focus left rehearsal window; refusing to type into {active.get('title')!r}")
    subprocess.run(["wtype", *args], check=True)


def keys(text, wait=0.6):
    """Type text one character at a time; "\\n" is a Return key."""
    for ch in text:
        if VERBOSE:
            log("  · type " + ("⏎" if ch == "\n" else ch))
        if ch == "\n":
            wtype("-k", "Return")
            time.sleep(0.35)
        else:
            wtype(ch)
            time.sleep(CHAR_GAP)
    time.sleep(wait)


def key(mods, k, wait=0.6):
    """A chord: mods is a list of wtype modifier names (ctrl, alt, shift, super)."""
    if VERBOSE:
        log("  · " + "+".join(mods + [k]))
    args = []
    for m in mods:
        args += ["-M", m]
    args += ["-k", k]
    for m in reversed(mods):
        args += ["-m", m]
    wtype(*args)
    time.sleep(wait)


def esc(wait=0.6):
    key([], "Escape", wait)


def focus(cls, wait=2.0, pid=None):
    global target_address
    if VERBOSE:
        log("  · focus " + cls)
    windows = json.loads(subprocess.check_output(["hyprctl", "clients", "-j"], text=True))
    matches = [w for w in windows if re.fullmatch(cls, w["class"]) and (pid is None or w["pid"] == pid)]
    if len(matches) != 1:
        raise RuntimeError(f"Expected one {cls!r} window, found {len(matches)}; refusing to type")
    address = matches[0]["address"]
    args = [f'hl.dsp.focus({{window="address:{address}"}})'] if LUA else ["focuswindow", f"address:{address}"]
    subprocess.run(["hyprctl", "dispatch", *args], capture_output=True, check=True)
    time.sleep(wait)
    active = json.loads(subprocess.check_output(["hyprctl", "activewindow", "-j"], text=True))
    if active.get("address") != address:
        raise RuntimeError(f"Could not focus {cls!r}; refusing to type")
    target_address = address


def expect(mode, reason=None):
    st = status()
    ok_mode = st.get("mode") == mode
    ok_reason = reason is None or reason in str(st.get("reason", ""))
    ok = ok_mode and ok_reason
    verdict = "PASS" if ok else "FAIL"
    results["pass" if ok else "fail"] += 1
    want = f"mode={mode}" + (f" reason~{reason}" if reason else "")
    log(f'{verdict}  want {want}  got mode={st.get("mode")} code={st.get("code")} reason="{st.get("reason")}"')


def open_demo_ghostty():
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
    sh(f"code '{SHOW}/demo-go.code-workspace'", 5)
    focus("code", 1)
    key(["ctrl"], "p", 0.8); keys("modes.go", 0.6); key([], "Return", 2.0)
    esc(0.8); expect("normal", "client")
    vim_tour()
    keys("/Kana\n", 1.0); expect("normal")
    keys("A", 0.5); expect("insert")
    keys(" // bit 1 of the code", 0.4)
    esc(0.5); expect("normal")
    keys("v", 0.5); expect("visual")
    esc(0.5); expect("normal")
    key(["ctrl"], "grave", 1.5); expect("raw", "tool window")
    keys("go run ./cmd/vimmode\n", 2.5); expect("raw", "tool window")
    key(["ctrl"], "grave", 1.2); expect("normal")
    key([], "F1", 1.2); expect("raw")
    keys("keyboard", 0.9); expect("raw")
    esc(0.8); expect("normal")
    key(["ctrl", "shift"], "e", 1.2); expect("raw", "tool window")
    key([], "Down", 0.4); key([], "Down", 0.5); expect("raw")
    key(["ctrl", "shift"], "e", 1.2); expect("normal")
    keys("u", 0.4)


def seg6():
    log("--- segment 6: IntelliJ IDEA")
    focus("jetbrains-idea", 2)
    key(["ctrl", "shift"], "n", 1.0); keys("ModeTable", 0.8); key([], "Return", 2.0)
    expect("normal", "intellij")          # no Esc: IdeaVim beeps on Esc in normal mode
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
    focus("obsidian|md\\.obsidian\\.Obsidian", 1)
    esc(0.8); expect("normal", "obsidian")
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
    which = next((a for a in sys.argv[1:] if not a.startswith("--")), "all")
    order = list(SEGMENTS) if which == "all" else [which]
    log(f"rehearsal {which} — hands off the keyboard")
    print("starting in 3 seconds…")
    time.sleep(3)
    for k in order:
        if k in SEGMENTS:
            time.sleep(1.5)
            SEGMENTS[k]()
        else:
            log(f"unknown segment {k}")
    log(f"DONE  pass={results['pass']} fail={results['fail']}")
    sys.exit(1 if results["fail"] else 0)
