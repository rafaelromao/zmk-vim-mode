#!/usr/bin/env python3
"""Post-production for the showcase takes: lay the narration on the picture, in sync.

Unlike record.py and rehearse.py this runs anywhere — it is ffmpeg and a TTS binary,
with no gpu-screen-recorder, ydotool or hyprctl — so the dub can be cut on the laptop
while the takes are recorded on the Linux box.

    dub.py anchors          re-measure each take's first on-screen action
    dub.py resync           rebuild the bed from the EXISTING take<N>.wav files
    dub.py render [voice]   render SCRIPT.md's narration through the TTS engine
    dub.py master           lay the bed, normalise it, mux onto showcase-takes.mp4
    dub.py hud [N...]       what layer the HUD shows, when — the timeline cues go against
    dub.py fit              does the narration fit the picture? (works without TTS)
    dub.py check            report sync, loudness and speech density of the output

Why a single bed instead of ten padded clips: every cue is placed at its absolute
position in the assembly, so a wrong length anywhere cannot push anything after it.
The blanket `adelay=9000` this replaces was a take-level delay, and it put every beat
3.7-5.2 s behind its own picture because takes 0-3 open with 5.2 s of nothing and
takes 4-9 with 3.9 s.

The picture is never re-encoded: the master is muxed `-c:v copy`.
"""

import json
import os
import shutil
import subprocess
import sys

SHOW = os.path.dirname(os.path.abspath(__file__))
RUN = os.path.join(SHOW, "run")
ANCHORS = os.path.join(RUN, "anchors.json")
ASSEMBLY = os.path.join(RUN, "showcase-takes.mp4")
BED = os.path.join(RUN, "narration.wav")
OUT = os.path.join(RUN, "showcase-dubbed.mp4")
CLIPS = os.path.join(RUN, "clips")

BEATS = [str(n) for n in range(10)]
# The voice starts a beat fractionally before the keystroke it describes; landing
# exactly on it reads as late.
LEAD = float(os.environ.get("ZMK_DUB_LEAD", "0.3"))
# obs-scene.md's delivery spec.
TARGET_I, TARGET_TP, TARGET_LRA = "-16", "-1.5", "11"
RATE, CHANNELS = "48000", "2"

sys.path.insert(0, SHOW)
import narration  # noqa: E402


def fail(msg):
    print(f"dub: {msg}", file=sys.stderr, flush=True)
    sys.exit(2)


def ff(args, **kw):
    return subprocess.run(["ffmpeg", "-v", "error", "-y", *args],
                          check=True, **kw)


def probe(path, entry="format=duration"):
    out = subprocess.run(["ffprobe", "-v", "error", "-show_entries", entry,
                          "-of", "csv=p=0", path],
                         capture_output=True, text=True).stdout.strip()
    return out


def duration(path):
    return float(probe(path))


# ---------------------------------------------------------------- anchors

def scene_hits(path, thresh, window=30):
    out = subprocess.run(
        ["ffmpeg", "-hide_banner", "-nostats", "-t", str(window), "-i", path,
         "-vf", f"select='gt(scene,{thresh})',showinfo", "-f", "null", "-"],
        capture_output=True, text=True).stderr
    return [float(l.split("pts_time:")[1].split()[0])
            for l in out.splitlines() if "pts_time:" in l]


def measure_anchors():
    """First sustained change per take. A blinking cursor trips the loose threshold
    but never the firm one, so a loose hit only counts as the action if a firm hit
    stands with it or after it."""
    anchors, acc = {}, 0.0
    for b in BEATS:
        mp4 = os.path.join(RUN, f"take{b}.mp4")
        if not os.path.isfile(mp4):
            fail(f"missing {mp4}")
        loose, firm = scene_hits(mp4, 0.02), scene_hits(mp4, 0.04)
        action = next((t for t in loose
                       if any(abs(f - t) < 0.01 for f in firm) or any(f > t for f in firm)),
                      loose[0] if loose else 0.0)
        wav = os.path.join(RUN, f"take{b}.wav")
        anchors[b] = {
            "action": round(action, 3),
            "video": round(duration(mp4), 3),
            "narration": round(duration(wav), 3) if os.path.isfile(wav) else None,
            "offset": round(acc, 3),
        }
        acc += anchors[b]["video"]
    json.dump(anchors, open(ANCHORS, "w"), indent=2)
    return anchors


def anchors():
    if not os.path.isfile(ANCHORS):
        return measure_anchors()
    return json.load(open(ANCHORS))


# ---------------------------------------------------------------- the HUD's own timeline

# The layer HUD's banner sits in the right rail, and its underline is a solid bar whose
# colour IS the layer: green for insert, bright blue for the vim command layers, dim
# blue-grey for Alpha 1 (raw and off both read Alpha 1). Cropping that bar and averaging
# it to one pixel turns "what was the keyboard doing" into three bytes per sample.
#
# This is what a cue should be placed against. Scene detection on the whole frame finds
# only that *something* moved; scene detection on the banner misses insert→normal, because
# the text is the same length and only the colour changes. The colour is the signal.
HUD_CROP = "crop=700:6:1816:126"
HUD_FPS = 4.0


def hud_state(r, g, b):
    s = r + g + b
    if s < 110:
        return "none"       # HUD not drawn yet
    if g >= b:
        return "insert"     # green
    if b >= 115:
        return "normal"     # bright blue: normal, visual and cmdline all read blue here
    return "raw"            # dim blue-grey: Alpha 1


def hud_timeline(beat, minhold=0.75):
    """[(start, end, state)] through take<beat>, in take-relative seconds."""
    mp4 = os.path.join(RUN, f"take{beat}.mp4")
    raw = subprocess.run(
        ["ffmpeg", "-v", "error", "-i", mp4, "-vf", f"{HUD_CROP},fps={HUD_FPS:g},scale=1:1",
         "-f", "rawvideo", "-pix_fmt", "rgb24", "-"], capture_output=True).stdout
    states = [hud_state(*raw[i:i + 3]) for i in range(0, len(raw), 3)]
    if not states:
        return []
    segs, cur, start = [], states[0], 0.0
    for i, st in enumerate(states[1:], 1):
        if st != cur:
            segs.append((start, i / HUD_FPS, cur))
            cur, start = st, i / HUD_FPS
    segs.append((start, len(states) / HUD_FPS, cur))
    out = []
    for a_, b_, st in segs:                       # fold away blips and merge neighbours
        if out and b_ - a_ < minhold:
            out[-1] = (out[-1][0], b_, out[-1][2])
        elif out and out[-1][2] == st:
            out[-1] = (out[-1][0], b_, st)
        else:
            out.append((a_, b_, st))
    return out


def hud_at(timeline, t):
    for a_, b_, st in timeline:
        if a_ <= t < b_:
            return st
    return "—"


# ---------------------------------------------------------------- TTS engines

def kokoro_render(text, wav, voice):
    """kokoro-onnx in the venv the README's setup builds."""
    venv = os.path.expanduser(os.environ.get(
        "ZMK_KOKORO_VENV", "~/.cache/zmk-showcase/tts-kokoro"))
    py = os.path.join(venv, "bin", "python")
    voices = os.path.expanduser("~/.cache/zmk-showcase/voices")
    model = os.path.join(voices, "kokoro-v1.0.onnx")
    bins = os.path.join(voices, "voices-v1.0.bin")
    if not os.path.isfile(py):
        return None
    if not (os.path.isfile(model) and os.path.isfile(bins)):
        fail(f"kokoro venv present but no model in {voices}")
    prog = (
        "import sys, soundfile as sf\n"
        "from kokoro_onnx import Kokoro\n"
        "k = Kokoro(sys.argv[1], sys.argv[2])\n"
        "s, sr = k.create(sys.stdin.read(), voice=sys.argv[4], speed=float(sys.argv[5]), lang='en-us')\n"
        "sf.write(sys.argv[3], s, sr)\n")
    speed = os.environ.get("ZMK_DUB_SPEED", "1.0")
    subprocess.run([py, "-c", prog, model, bins, wav, voice, speed],
                   input=text.encode(), check=True)
    return wav


def piper_render(text, wav, voice):
    binary = shutil.which("piper") or os.path.expanduser(
        "~/.cache/zmk-showcase/tts/bin/piper")
    if not os.path.isfile(binary) and not shutil.which("piper"):
        return None
    model = os.path.expanduser(os.environ.get(
        "ZMK_TTS_VOICE", f"~/.cache/zmk-showcase/voices/{voice}.onnx"))
    if not os.path.isfile(model):
        fail(f"piper present but no voice at {model}")
    scale = os.environ.get("ZMK_TTS_LENGTH_SCALE", "1.4")
    subprocess.run([binary, "--model", model, "--length-scale", scale,
                    "--output_file", wav],
                   input=text.encode(), check=True,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    return wav


def render_one(text, wav, voice):
    for engine in (kokoro_render, piper_render):
        got = engine(text, wav, voice)
        if got:
            return got
    fail("no TTS engine: install kokoro-onnx or piper (see README 'Dubbing')")


def render(voice=None):
    """Render every cue of every beat to run/clips/beat<N>-<i>.wav."""
    voice = voice or os.environ.get("ZMK_DUB_VOICE", "am_michael")
    shutil.rmtree(CLIPS, ignore_errors=True)
    os.makedirs(CLIPS, exist_ok=True)
    made = []
    for b in BEATS:
        for i, (cue, text) in enumerate(narration.cues(b)):
            wav = os.path.join(CLIPS, f"beat{b}-{i:02d}.wav")
            render_one(text, wav, voice)
            made.append((b, i, cue, wav, duration(wav)))
            print(f"  beat {b} cue {i:02d} "
                  f"{'@+%.1f' % cue if cue is not None else '@start'}  "
                  f"{duration(wav):5.1f}s  {text[:60]}")
    print(f"rendered {len(made)} clips with {voice}")
    return made


# ---------------------------------------------------------------- the bed

def placements(source):
    """[(absolute_seconds, wav)] for the whole assembly.

    source 'takes'  — the existing per-beat take<N>.wav, one clip per beat
    source 'clips'  — the freshly rendered per-cue clips, placed at their cues
    """
    a, out = anchors(), []
    for b in BEATS:
        base = a[b]["offset"] + a[b]["action"] - LEAD
        if source == "takes":
            wav = os.path.join(RUN, f"take{b}.wav")
            if os.path.isfile(wav):
                out.append((max(0.0, base), wav))
            continue
        for i, (cue, _) in enumerate(narration.cues(b)):
            wav = os.path.join(CLIPS, f"beat{b}-{i:02d}.wav")
            if not os.path.isfile(wav):
                fail(f"missing {wav}; run `dub.py render` first")
            at = base + (cue or 0.0)
            out.append((max(0.0, at), wav))
    return sorted(out)


def build_bed(source):
    """One track the length of the assembly, every clip at its absolute position."""
    place = placements(source)
    if not place:
        fail("nothing to place")
    total = duration(ASSEMBLY)
    args, filters, labels = [], [], []
    for n, (at, wav) in enumerate(place):
        args += ["-i", wav]
        filters.append(
            f"[{n}:a]aresample={RATE},aformat=channel_layouts=stereo,"
            f"adelay={int(round(at * 1000))}:all=1[d{n}]")
        labels.append(f"[d{n}]")
    chain = ";".join(filters)
    # normalize=0 keeps a clip at its own level instead of dividing by input count
    chain += (f";{''.join(labels)}amix=inputs={len(place)}:normalize=0:"
              f"dropout_transition=0[m];[m]apad,atrim=0:{total}[out]")
    ff([*args, "-filter_complex", chain, "-map", "[out]",
        "-c:a", "pcm_s16le", "-ar", RATE, "-ac", CHANNELS, BED])
    print(f"bed: {len(place)} clips over {duration(BED):.3f}s (assembly {total:.3f}s)")
    return BED


def loudnorm(src, dst):
    """Two-pass EBU R128. The one-pass filter cannot hit a true-peak ceiling, and
    the scratch track's +0.1..+0.3 dBTP clipping is half of why it sounded bad."""
    out = subprocess.run(
        ["ffmpeg", "-v", "info", "-i", src, "-af",
         f"loudnorm=I={TARGET_I}:TP={TARGET_TP}:LRA={TARGET_LRA}:print_format=json",
         "-f", "null", "-"], capture_output=True, text=True).stderr
    blob = out[out.rindex("{"):out.rindex("}") + 1]
    m = json.loads(blob)
    ff(["-i", src, "-af",
        f"loudnorm=I={TARGET_I}:TP={TARGET_TP}:LRA={TARGET_LRA}:"
        f"measured_I={m['input_i']}:measured_TP={m['input_tp']}:"
        f"measured_LRA={m['input_lra']}:measured_thresh={m['input_thresh']}:"
        f"offset={m['target_offset']}:linear=true:print_format=summary",
        "-ar", RATE, "-ac", CHANNELS, "-c:a", "pcm_s16le", dst])
    print(f"loudnorm: in I={m['input_i']} TP={m['input_tp']} -> target {TARGET_I}/{TARGET_TP}")
    return dst


def master(source="clips"):
    build_bed(source)
    normed = os.path.join(RUN, "narration-normed.wav")
    loudnorm(BED, normed)
    ff(["-i", ASSEMBLY, "-i", normed,
        "-map", "0:v", "-map", "1:a", "-c:v", "copy",
        "-c:a", "aac", "-b:a", "192k", "-ar", RATE, "-ac", CHANNELS,
        "-movflags", "+faststart", "-shortest", OUT])
    print(f"wrote {OUT} ({duration(OUT):.3f}s)")
    return OUT


# ---------------------------------------------------------------- check

def check(path=OUT):
    if not os.path.isfile(path):
        fail(f"no {path}")
    a = anchors()
    out = subprocess.run(
        ["ffmpeg", "-hide_banner", "-nostats", "-i", path, "-map", "0:a",
         "-af", "silencedetect=noise=-45dB:d=0.6", "-f", "null", "-"],
        capture_output=True, text=True).stderr
    ends = [float(l.split("silence_end:")[1].split()[0])
            for l in out.splitlines() if "silence_end:" in l]
    starts = [float(l.split("silence_start:")[1].split()[0])
              for l in out.splitlines() if "silence_start:" in l]

    print("\nsync — first speech in each beat vs its measured action:")
    worst = 0.0
    for b in BEATS:
        want = a[b]["offset"] + a[b]["action"] - LEAD
        seg = [e for e in ends if e >= a[b]["offset"] - 0.5]
        got = seg[0] if seg else float("nan")
        err = got - want
        worst = max(worst, abs(err))
        flag = "ok" if abs(err) <= 0.30 else "OFF"
        print(f"  beat {b}  want {want:7.2f}  got {got:7.2f}  err {err:+5.2f}s  {flag}")
    print(f"  worst |err| = {worst:.2f}s")

    print("\ndensity — speech vs picture per beat:")
    for b in BEATS:
        lo, hi = a[b]["offset"], a[b]["offset"] + a[b]["video"]
        quiet = 0.0
        for s, e in zip(starts, ends + [duration(path)]):
            if e <= lo or s >= hi:
                continue
            quiet += min(e, hi) - max(s, lo)
        d = 100.0 * (1 - quiet / a[b]["video"])
        print(f"  beat {b}  {d:5.1f}%")

    r = subprocess.run(["ffmpeg", "-hide_banner", "-nostats", "-i", path,
                        "-af", "ebur128=peak=true", "-f", "null", "-"],
                       capture_output=True, text=True).stderr.splitlines()
    tail = "\n".join(r[-14:])
    print("\nloudness (EBU R128):")
    for line in tail.splitlines():
        if any(k in line for k in ("I:", "LRA:", "Peak:")):
            print("  " + line.strip())


# ---------------------------------------------------------------- fit

WPM = float(os.environ.get("ZMK_DUB_WPM", "140"))


def clip_seconds(text, rendered=None):
    """A clip's length: measured if it has been rendered, else estimated at WPM."""
    if rendered and os.path.isfile(rendered):
        return duration(rendered)
    return len(text.split()) * 60.0 / WPM


def fit(strict=False):
    """Does the narration fit the picture? Flags a cue that would still be talking
    when the next one starts, and a beat that runs past the end of its take.

    Works before any TTS exists (estimating from word count at WPM) and again after
    (measuring the rendered clips), so the script can be written and checked on a
    machine with no voice installed.
    """
    a, bad = anchors(), 0
    total_speech = total_video = 0.0
    for b in BEATS:
        cs = narration.cues(b)
        tl = hud_timeline(b)
        usable = a[b]["video"] - a[b]["action"] + LEAD
        spoken = 0.0
        print(f"\nbeat {b} — take {a[b]['video']:.1f}s, action at "
              f"{a[b]['action']:.1f}s, {usable:.1f}s usable")
        for i, (cue, text) in enumerate(cs):
            at = (cue or 0.0)
            wav = os.path.join(CLIPS, f"beat{b}-{i:02d}.wav")
            dur = clip_seconds(text, wav)
            spoken += dur
            end = at + dur
            nxt = cs[i + 1][0] if i + 1 < len(cs) and cs[i + 1][0] is not None else None
            note = ""
            if nxt is not None and end > nxt + 0.05:
                note, bad = f"  OVERLAPS next by {end - nxt:.1f}s", bad + 1
            elif end > usable + 0.05:
                note, bad = f"  RUNS PAST take end by {end - usable:.1f}s", bad + 1
            src = "meas" if os.path.isfile(wav) else "est"
            # the layer the HUD is actually showing while this line is spoken
            st0 = hud_at(tl, at + a[b]["action"] - LEAD)
            st1 = hud_at(tl, end + a[b]["action"] - LEAD)
            span = st0 if st0 == st1 else f"{st0}→{st1}"
            print(f"  +{at:5.1f} → {end:5.1f}  {dur:4.1f}s ({src}, "
                  f"{len(text.split()):3d}w)  HUD {span:<14}{note}")
        total_speech += spoken
        total_video += a[b]["video"]
        print(f"  spoken {spoken:.1f}s of {a[b]['video']:.1f}s "
              f"= {100 * spoken / a[b]['video']:.0f}% density")
    print(f"\noverall: {total_speech:.0f}s speech over {total_video:.0f}s "
          f"= {100 * total_speech / total_video:.0f}% density")
    print(f"{bad} problem(s)")
    if strict and bad:
        sys.exit(1)
    return bad


USAGE = __doc__


def main():
    cmd = sys.argv[1] if len(sys.argv) > 1 else ""
    if cmd == "anchors":
        for b, d in measure_anchors().items():
            print(f"take{b}  action={d['action']:>7.3f}  video={d['video']:>7.3f}  "
                  f"offset={d['offset']:>8.3f}")
    elif cmd == "resync":
        master(source="takes")
    elif cmd == "render":
        render(sys.argv[2] if len(sys.argv) > 2 else None)
    elif cmd == "master":
        master(source="clips")
    elif cmd == "hud":
        for b in (sys.argv[2:] or BEATS):
            print(f"=== take{b} (anchor {anchors()[b]['action']:.2f}s) ===")
            for a_, b_, st in hud_timeline(b):
                if st == "none":
                    continue
                an = anchors()[b]["action"]
                print(f"  {a_:6.2f} → {b_:6.2f}   cue +{a_ - an:6.2f} → +{b_ - an:6.2f}   {st}")
    elif cmd == "fit":
        fit(strict="--strict" in sys.argv)
    elif cmd == "check":
        check(sys.argv[2] if len(sys.argv) > 2 else OUT)
    else:
        print(USAGE)
        sys.exit(2)


if __name__ == "__main__":
    main()
