#!/usr/bin/env python3
"""Automated takes for the showcase video: screen-record rehearsal segments with
a TTS scratch narration track.

    python3 showcase/record.py [0-9|all] [--no-tts]

One take per beat lands in showcase/run/take<N>.mp4 (gitignored): the segment's
screen output on HDMI-A-1 at 30 fps (rehearsal HUD included, exactly as the
rehearsal runs it) with the beat's SCRIPT.md narration read over it. The voice
is a scratch track for judging pacing — the deliverable is dubbed by hand.

Needs: gpu-screen-recorder (CPU fallback, NVENC is too old on this box),
ffmpeg, piper-tts + a voice, everything the rehearsal needs. Voices live in
~/.cache/zmk-showcase/voices (override with ZMK_TTS_VOICE); the piper binary
is looked up on PATH, then $ZMK_TTS_BIN:

    python3 -m venv ~/.cache/zmk-showcase/tts && \
      ~/.cache/zmk-showcase/tts/bin/pip install piper-tts
    mkdir -p ~/.cache/zmk-showcase/voices && cd $_ &&
      curl -sLO https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx{,.json}

Run showcase/prepare.sh first and keep hands off while it runs. A take that
fails stops at the failed segment like the rehearsal does; the driver keeps
the raw capture and moves on to the next beat, so one flake does not eat the
session. Takes are silent re-runs of the rehearsal: stop the take's HUD first
(`bash showcase/hud.sh stop`), the rehearsal brings its own.
"""

import os
import shutil
import signal
import subprocess
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import narration as narration_md   # noqa: E402  (needs the path above)

SHOW = os.path.dirname(os.path.abspath(__file__))
RUN = os.path.join(SHOW, "run")
REHEARSE = os.path.join(SHOW, "rehearse.py")
SCRIPT = os.path.join(SHOW, "SCRIPT.md")
MONITOR = os.environ.get("ZMK_RECORD_MONITOR", "HDMI-A-1")
FPS = os.environ.get("ZMK_RECORD_FPS", "30")
BEATS = [str(n) for n in range(10)]   # every beat of SCRIPT.md is a segment of rehearse.py
# TTS at the script's assumed narration pace (~140 words per minute on the ryan
# voice needs length-scale ~1.77; 1.3 rendered ~165wpm and crammed every beat's
# words into its first third). Post still refits + delays the scratch per take.
LENGTH_SCALE = os.environ.get("ZMK_TTS_LENGTH_SCALE", "1.77")

os.makedirs(RUN, exist_ok=True)


def fail(msg):
    print(f"record: {msg}", file=sys.stderr, flush=True)
    sys.exit(2)


def narration(beat):
    """The beat's narration as one string, for the scratch track.

    narration.py owns the parsing so this and dub.py can never disagree about what a
    beat says. Cues (`[+12.3]`) are stripped here: the scratch only judges pace, and
    dub.py is what places each line at its own moment.
    """
    return narration_md.flat_text(beat, SCRIPT)


def tts_bin():
    for cand in (shutil.which("piper"), os.environ.get("ZMK_TTS_BIN")):
        if cand and os.access(cand, os.X_OK):
            return cand
    fail("no piper binary: install piper-tts or set ZMK_TTS_BIN")


def voice():
    cand = os.environ.get("ZMK_TTS_VOICE",
                           os.path.expanduser("~/.cache/zmk-showcase/voices/en_US-ryan-medium.onnx"))
    if os.path.isfile(cand):
        return cand
    fail(f"no voice at {cand} (see this script's docstring)")


def render_tts(text, wav):
    subprocess.run([tts_bin(), "--model", voice(),
                    "--length-scale", str(LENGTH_SCALE),
                    "--output_file", wav],
                   input=text.encode(), check=True,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def start_recorder(raw):
    log = open(os.path.join(RUN, "record-gsr.log"), "a")
    proc = subprocess.Popen(
        ["gpu-screen-recorder", "-w", MONITOR, "-f", FPS, "-cursor", "yes",
         "-fallback-cpu-encoding", "yes", "-o", raw],
        stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
    time.sleep(2)
    if proc.poll() is not None:
        fail(f"recorder died at once; see {RUN}/record-gsr.log")
    return proc


def stop_recorder(proc):
    if proc.poll() is not None:
        return
    proc.send_signal(signal.SIGINT)
    try:
        proc.wait(timeout=20)
    except subprocess.TimeoutExpired:
        proc.terminate()
        proc.wait(timeout=10)


def run_segment(seg):
    return subprocess.run([sys.executable, "-u", REHEARSE, seg]).returncode


def kill_strays():
    """Orphaned rehearsal HUDs (a killed driver never lets rehearse.py run its
    cleanup): precise path patterns only, never broad process names."""
    for pat in ("showcase/rehearsal-panel.py", "showcase/rehearsal-feed.py"):
        subprocess.run(["pkill", "-f", pat], capture_output=True)
    time.sleep(1)


def set_idle(mode):
    """omarchy idle mode (stay-awake|allow-idle); warns instead of failing."""
    if shutil.which("omarchy") is None:
        print("record: omarchy not found; screensaver may fire mid-take", flush=True)
        return
    subprocess.run(["omarchy", "toggle", "idle", mode],
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def idle_active():
    """True when stay-awake is already on (leave it alone then)."""
    if shutil.which("omarchy") is None:
        return True
    try:
        out = subprocess.run(["omarchy", "toggle", "idle", "status"],
                             capture_output=True, text=True, timeout=10).stdout
        import json
        return bool(json.loads(out).get("enabled"))
    except Exception:
        return True


def mux(raw, wav, out):
    """Video frozen briefly past its end + narration padded: take length = max,
    without minutes of dead tail."""
    subprocess.run(
        ["ffmpeg", "-v", "error", "-y", "-i", raw, "-i", wav,
         "-filter_complex",
         "[0:v]tpad=stop_mode=clone:stop_duration=5[v];[1:a]apad[a]",
         "-map", "[v]", "-map", "[a]", "-c:v", "libx264", "-preset", "veryfast",
         "-crf", "23", "-c:a", "aac", "-shortest", out],
        check=True)
    os.remove(raw)


# Takes never show the mode line: beats with no editor state would only read
# "off — terminal without nvim client", and beats 3 and 8 carry the reason in
# typed `status` lines instead. `modeline.sh` stays for manual debugging.
def modeline(action):
    subprocess.run(["bash", os.path.join(SHOW, "modeline.sh"), action],
                   capture_output=True)


def park():
    """Clear stale demo terminals and focus workspace 8 before a take.

    A previous attempt can leave its Ghostty surface on workspace 8. Since the
    recorder starts before the segment opens its fresh demo terminal, that old
    screen leaks into the head of the take. Workspace 8 is reserved for this
    showcase terminal, so close only Ghostty windows there before recording.
    """
    clients = json.loads(subprocess.check_output(["hyprctl", "clients", "-j"], text=True))
    for window in clients:
        workspace = window.get("workspace") or {}
        if window.get("class") != "com.mitchellh.ghostty" or workspace.get("id") != 8:
            continue
        address = window.get("address")
        if address:
            subprocess.run(
                ["hyprctl", "eval",
                 f"return hl.dispatch(hl.dsp.window.close({{window='address:{address}'}}))"],
                capture_output=True, check=True)
    deadline = time.time() + 5
    while time.time() < deadline:
        clients = json.loads(subprocess.check_output(["hyprctl", "clients", "-j"], text=True))
        stale = [w for w in clients
                 if w.get("class") == "com.mitchellh.ghostty"
                 and (w.get("workspace") or {}).get("id") == 8]
        if not stale:
            break
        time.sleep(0.1)
    else:
        fail("stale Ghostty window is still on workspace 8; refusing a contaminated take")
    subprocess.run(["hyprctl", "dispatch", 'hl.dsp.focus({workspace="8"})'],
                   capture_output=True)


def take(seg, with_tts):
    raw = os.path.join(RUN, f"take{seg}-raw.mp4")
    out = os.path.join(RUN, f"take{seg}.mp4")
    wav = os.path.join(RUN, f"take{seg}.wav")
    for f in (raw, out):
        if os.path.exists(f):
            os.remove(f)
    text = narration(seg) if with_tts else ""
    if text:
        print(f"take {seg}: rendering narration", flush=True)
        render_tts(text, wav)
    else:
        wav = None
    print(f"take {seg}: recording", flush=True)
    park()
    modeline("stop")
    rec = start_recorder(raw)
    try:
        time.sleep(1)
        rc = run_segment(seg)
        time.sleep(1)
    finally:
        stop_recorder(rec)
    if rc != 0:
        print(f"take {seg}: segment failed (rc={rc}); raw kept at {raw}", flush=True)
        return False
    if wav is None:
        os.rename(raw, out)
    else:
        print(f"take {seg}: muxing", flush=True)
        mux(raw, wav, out)
    print(f"take {seg}: done -> {out}", flush=True)
    return True


def feed_python():
    """The interpreter rehearsal-panel.py will run the feed with (same rule).
    Heals a rebuilt venv (evdev is a showcase need the HUD's Makefile does not
    know about) and fails fast only when that is impossible."""
    import pathlib
    hud = pathlib.Path(os.environ.get("ZMK_LAYER_HUD",
                                       os.path.expanduser("~/projects/zmk-layer-hud")))
    cand = os.environ.get("ZMKHUD_PYTHON") or str(hud / ".venv/bin/python3")
    if not (cand and os.access(cand, os.X_OK)):
        return None

    def deps_ok():
        return subprocess.run(
            [cand, "-c", "import evdev, serial, websockets"],
            capture_output=True).returncode == 0

    if not deps_ok():
        # A rebuild wipes it; a corrupt one (dist-info without files) fools a
        # plain install into a no-op — escalate to --force-reinstall then.
        pip = os.path.join(os.path.dirname(cand), "pip")
        print("record: feed venv lost a dep (rebuilt?); reinstalling evdev", flush=True)
        subprocess.run([pip, "install", "--quiet", "evdev"],
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        if not deps_ok():
            subprocess.run([pip, "install", "--quiet",
                            "--force-reinstall", "--no-cache-dir", "evdev"],
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    return cand if deps_ok() else None


if __name__ == "__main__":
    if "--help" in sys.argv or "-h" in sys.argv:
        print(__doc__.strip().split("\n\n")[0])
        print("usage: python3 showcase/record.py [0-9|all] [--no-tts]")
        sys.exit(0)
    unknown = [a for a in sys.argv[1:] if a.startswith("--") and a != "--no-tts"]
    if unknown:
        fail(f"unknown flags: {' '.join(unknown)}")
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    with_tts = "--no-tts" not in sys.argv
    which = next(iter(args), "all")
    segs = list(BEATS) if which == "all" else [which]
    if any(s not in BEATS for s in segs):
        fail(f"unknown beat {which}; want one of {BEATS} or all")
    if shutil.which("gpu-screen-recorder") is None:
        fail("gpu-screen-recorder not found")
    if shutil.which("ffmpeg") is None:
        fail("ffmpeg not found")
    if feed_python() is None:
        fail("rehearsal feed interpreter lacks evdev/serial/websockets; "
             "install them in zmk-layer-hud/.venv (pip install evdev)")
    if with_tts:
        tts_bin()
        voice()
    print(f"recording beats {','.join(segs)} — hands off", flush=True)
    kill_strays()
    hold_idle = not idle_active()
    if hold_idle:
        print("record: stay-awake for the run", flush=True)
        set_idle("stay-awake")
    ok = True
    try:
        for seg in segs:
            ok = take(seg, with_tts) and ok
    finally:
        if hold_idle:
            set_idle("allow-idle")
            print("record: idle behavior restored", flush=True)
    sys.exit(0 if ok else 1)
