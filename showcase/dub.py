#!/usr/bin/env python3
"""Post-production for the showcase takes: lay the narration on the picture, in sync.

Unlike record.py and rehearse.py this runs anywhere — it is ffmpeg and a TTS binary,
with no gpu-screen-recorder, ydotool or hyprctl — so the dub can be cut on the laptop
while the takes are recorded on the Linux box.

    dub.py assemble         join the takes into showcase-takes.mp4 and prove it matches them
    dub.py anchors          re-measure each take's first on-screen action
    dub.py render [voice]   render SCRIPT.md's narration through the TTS engine
    dub.py master           lay the bed, normalise it, mux onto showcase-takes.mp4
    dub.py hud [N...]       what layer the HUD shows, when — the timeline cues go against
    dub.py fit              does the narration fit the picture? (works without TTS)
    dub.py keys N [every]   stack the typed-keys strip through take N — the ground truth
                            for cueing a line that names a keystroke
    dub.py proof N [file]   one panel per line of beat N: screen + typed keys at the
                            moment it is spoken, straight out of the finished video
    dub.py pauses           find the stretches that are both silent and frozen
    dub.py tighten          cut those out, re-place the narration → showcase.mp4
    dub.py check            report sync, loudness and speech density of the output

Why a single bed instead of ten padded clips: every cue is placed at its absolute
position in the assembly, so a wrong length anywhere cannot push anything after it.
The blanket `adelay=9000` this replaces was a take-level delay, and it put every beat
3.7-5.2 s behind its own picture because takes 0-3 open with 5.2 s of nothing and
takes 4-9 with 3.9 s.

The master is muxed `-c:v copy` unless the script has holds; `tighten` always re-encodes.

Holds (`**Hold** +16.3 for 5.5 s` in a beat of SCRIPT.md) freeze a still frame so a line
that is longer than its picture keeps its place. Every placement goes through one clock,
`Edit`, which counts the assembly's real frames through the cuts and the holds, so the voice
lands on the frame it was written against with no drift from the recordings' 59.99 fps.
"""

import bisect
import hashlib
import json
import os
import shutil
import subprocess
import sys

SHOW = os.path.dirname(os.path.abspath(__file__))
RUN = os.path.join(SHOW, "run")
ANCHORS = os.path.join(RUN, "anchors.json")
# The ten takes joined by `dub.py assemble`: a video-only concat copy, so frame
# `offset + t` of the assembly IS frame `t` of the take, decoded bit for bit. It must be
# rebuilt whenever a take is re-recorded: on 2026-09-23 every take was recorded again but
# the assembly was not, and the dub was cut against a recording that no longer existed.
# `assemble` stores the takes' hashes in run/assembly.json and master/tighten refuse to run
# when they no longer match.
ASSEMBLY = os.path.join(RUN, "showcase-takes.mp4")
STAMP = os.path.join(RUN, "assembly.json")
BED = os.path.join(RUN, "narration.wav")
OUT = os.path.join(RUN, "showcase-untrimmed.mp4")  # debugging aid, not shipped
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
    """Per take: when the HUD rail is drawn, and when the take's OWN picture begins.

    These are not the same moment and the difference matters. A take starts recording
    several seconds before its segment does anything, and what is on screen in the
    meantime is whatever the *previous* beat left there. The rail is drawn first (at
    3.7-5.2 s), the take's own content arrives later (consistently ~10 s), and scene
    detection on the whole frame finds the rail — it even resizes the editor window, so
    cropping the rail away does not help.

    Cues are relative to `action` (the rail appearing) because that is what they were
    written against, but no beat's opening line may start before `content_start`, or it
    narrates the previous beat's screen. That mistake put every beat 5-6.6 s early.
    """
    anchors, acc = {}, 0.0
    for b in BEATS:
        mp4 = os.path.join(RUN, f"take{b}.mp4")
        if not os.path.isfile(mp4):
            fail(f"missing {mp4}")
        rail = _first_change(mp4, "700:900:1806:60")
        content = _first_change(mp4, "1780:1300:10:70", after=rail + 1.5)
        wav = os.path.join(RUN, f"take{b}.wav")
        anchors[b] = {
            "action": round(rail, 3),
            "content_start": round(content, 3),
            "video": round(duration(mp4), 3),
            "narration": round(duration(wav), 3) if os.path.isfile(wav) else None,
            "offset": round(acc, 3),
        }
        acc += anchors[b]["video"]
    json.dump(anchors, open(ANCHORS, "w"), indent=2)
    return anchors


def _first_change(mp4, crop, after=0.0, fps=10.0, w=128, h=72, thresh=3.0, window=50):
    """First frame-to-frame jump in a region, at or after `after`. Single decode, no seeking."""
    raw = subprocess.run(
        ["ffmpeg", "-v", "error", "-i", mp4, "-t", str(window),
         "-vf", f"crop={crop},fps={fps:g},scale={w}:{h}",
         "-f", "rawvideo", "-pix_fmt", "gray", "-"], capture_output=True).stdout
    sz = w * h
    n = len(raw) // sz
    for i in range(max(1, int(after * fps)), n):
        d = sum(abs(raw[(i - 1) * sz + j] - raw[i * sz + j]) for j in range(sz)) / sz
        if d > thresh:
            return i / fps
    return after


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
#
# The crop is rows 130-133, the underline itself, in the 2026-09-23 takes (the rail sits a
# few pixels lower than it did on 09-20, and the old rows 126-131 caught mostly background).
# It must start on an even row with an even height: 4:2:0 video rounds a crop's height, and
# a 3-row crop silently comes back as 2. The colour is averaged here, not with `scale=1:1`:
# this Mac's ffmpeg 9 segfaults writing 1x1 frames and can lose the buffered output with it.
HUD_CROP = "crop=700:4:1820:130"
HUD_W, HUD_H = 700, 4
HUD_FPS = 4.0


def hud_state(r, g, b):
    s = r + g + b
    if s < 150:
        return "none"       # HUD not drawn
    if g > b + 15 and g > r:
        return "insert"     # green
    if b > 175 and b - r > 30 and g > r:
        return "normal"     # bright blue: normal and visual
    if r > b + 30 and r > g:
        return "cmdline"    # orange
    return "raw"            # dim blue-grey, Alpha 1 (raw and off alike); purple one-key hops


def hud_timeline(beat, minhold=0.75):
    """[(start, end, state)] through take<beat>, in take-relative seconds."""
    mp4 = os.path.join(RUN, f"take{beat}.mp4")
    raw = subprocess.run(
        ["ffmpeg", "-v", "error", "-i", mp4, "-vf", f"{HUD_CROP},fps={HUD_FPS:g}",
         "-f", "rawvideo", "-pix_fmt", "rgb24", "-"], capture_output=True).stdout
    n, px = HUD_W * HUD_H * 3, HUD_W * HUD_H
    states = [hud_state(sum(raw[i:i + n:3]) / px, sum(raw[i + 1:i + n:3]) / px,
                        sum(raw[i + 2:i + n:3]) / px)
              for i in range(0, len(raw) - n + 1, n)]
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
    """Render one line; returns the engine that made it ("kokoro" or "piper")."""
    for engine in (kokoro_render, piper_render):
        if engine(text, wav, voice):
            return engine.__name__.split("_")[0]
    fail("no TTS engine: install kokoro-onnx or piper (see README 'Dubbing')")


def render(voice=None):
    """Render every cue of every beat to run/clips/beat<N>-<i>.wav."""
    voice = voice or os.environ.get("ZMK_DUB_VOICE", "am_michael")
    shutil.rmtree(CLIPS, ignore_errors=True)
    os.makedirs(CLIPS, exist_ok=True)
    made, engines = [], set()
    for b in BEATS:
        for i, (cue, text) in enumerate(narration.cues(b)):
            wav = os.path.join(CLIPS, f"beat{b}-{i:02d}.wav")
            engines.add(render_one(text, wav, voice))
            made.append((b, i, cue, wav, duration(wav)))
            print(f"  beat {b} cue {i:02d} "
                  f"{'@+%.1f' % cue if cue is not None else '@start'}  "
                  f"{duration(wav):5.1f}s  {text[:60]}")
    print(f"rendered {len(made)} clips with {voice} ({', '.join(sorted(engines))})")
    if "piper" in engines:
        # The 2026-09-23 master shipped with the scratch voice because the Linux box has no
        # kokoro and this fallback was silent.
        print("dub: WARNING — piper is the scratch voice; the deliverable is kokoro "
              "(am_michael). Dub on a machine with the kokoro venv.", file=sys.stderr)
    return made


# ---------------------------------------------------------------- the assembly

def file_hash(path):
    h = hashlib.sha1()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def take_hashes():
    return {b: file_hash(os.path.join(RUN, f"take{b}.mp4")) for b in BEATS}


def frame_count(path):
    return int(probe(path, "stream=nb_frames").split()[0])


def thumbs(path, indices, w=64, h=36):
    """Grey thumbnails of the frames at these display-order indices, one decode, no seeking."""
    sel = "+".join(f"eq(n\\,{k})" for k in sorted(indices))
    raw = subprocess.run(
        ["ffmpeg", "-v", "error", "-i", path, "-vf", f"select='{sel}',scale={w}:{h}",
         "-fps_mode", "passthrough", "-f", "rawvideo", "-pix_fmt", "gray", "-"],
        capture_output=True).stdout
    sz = w * h
    return [raw[i:i + sz] for i in range(0, len(raw), sz)]


def verify_assembly():
    """Frame n of take b must be frame (frames of the takes before b) + n of the assembly.

    Three frames per take — its first, one from the middle, one near its end — compared
    after decoding. A stream copy decodes bit for bit, so anything but zero means the
    assembly is not made of these takes.
    """
    wanted, cum = {}, 0
    for b in BEATS:
        n = frame_count(os.path.join(RUN, f"take{b}.mp4"))
        wanted[b] = [(i, cum + i) for i in (0, n // 2, n - 10)]
        cum += n
    if frame_count(ASSEMBLY) != cum:
        fail(f"the assembly has {frame_count(ASSEMBLY)} frames, the takes {cum}")
    got = thumbs(ASSEMBLY, [j for pairs in wanted.values() for _, j in pairs])
    k = 0
    for b in BEATS:
        mine = thumbs(os.path.join(RUN, f"take{b}.mp4"), [i for i, _ in wanted[b]])
        for (i, j), frame in zip(wanted[b], mine):
            diff = sum(abs(x - y) for x, y in zip(frame, got[k])) / len(frame)
            k += 1
            if diff > 0.0:
                fail(f"take{b} frame {i} is not assembly frame {j} (mean diff {diff:.2f})")
    print(f"assembly verified: {cum} frames, every take where its offset says")


def assemble():
    """Join the takes, video only, by stream copy — then prove the join is exact."""
    lst = os.path.join(RUN, ".takes.txt")
    with open(lst, "w") as f:
        for b in BEATS:
            f.write(f"file '{os.path.join(RUN, f'take{b}.mp4')}'\n")
    ff(["-f", "concat", "-safe", "0", "-i", lst, "-map", "0:v", "-c", "copy",
        "-movflags", "+faststart", ASSEMBLY])
    os.remove(lst)
    verify_assembly()
    json.dump({"takes": take_hashes(), "assembly": file_hash(ASSEMBLY)},
              open(STAMP, "w"), indent=1)
    print(f"wrote {ASSEMBLY} ({duration(ASSEMBLY):.3f}s) and {STAMP}")


def require_assembly():
    """Refuse to dub an assembly that is not made of the takes on disk."""
    if not os.path.isfile(STAMP):
        fail("no run/assembly.json: run `dub.py assemble` to rebuild and check the assembly")
    stamp = json.load(open(STAMP))
    if stamp.get("takes") != take_hashes():
        fail("a take changed since the assembly was built: run `dub.py assemble`")
    if stamp.get("assembly") != file_hash(ASSEMBLY):
        fail("showcase-takes.mp4 changed since `dub.py assemble` checked it: run it again")


# ---------------------------------------------------------------- the edit's clock

def line_starts():
    """[(beat, index, assembly_seconds, wav)] for every rendered cue, in script order."""
    a, out = anchors(), []
    for b in BEATS:
        base = a[b]["offset"] + a[b]["action"] - LEAD
        for i, (cue, _) in enumerate(narration.cues(b)):
            wav = os.path.join(CLIPS, f"beat{b}-{i:02d}.wav")
            if not os.path.isfile(wav):
                fail(f"missing {wav}; run `dub.py render` first")
            out.append((b, i, max(0.0, base + (cue or 0.0)), wav))
    return out


def hold_points():
    """[(assembly_seconds, hold_seconds)] for every beat's holds, earliest first."""
    a = anchors()
    return sorted((a[b]["offset"] + a[b]["action"] + cue, secs)
                  for b in BEATS for cue, secs in narration.holds(b))


def expand(t, holds):
    """A moment of the assembly → the same moment with the holds played out (no cuts)."""
    return t + sum(s for h, s in holds if h < t)


def contract(x, holds):
    """The inverse of expand: a moment inside a hold maps to the held frame's time."""
    shift = 0.0
    for h, s in holds:
        if x <= h + shift:
            break
        if x < h + shift + s:
            return h
        shift += s
    return x - shift


def frame_times(path=ASSEMBLY):
    """Presentation time of every video frame of `path`, in display order."""
    out = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v:0",
                          "-show_entries", "packet=pts_time", "-of", "csv=p=0", path],
                         capture_output=True, text=True).stdout.split()
    return sorted(float(x) for x in out if x.strip() and x.strip() != "N/A")


class Edit:
    """The finished video's clock: which frames survive, which repeat, where a moment lands.

    `select` keeps the frames outside the cuts, `setpts` renumbers them at 60 fps leaving
    `n` empty slots after each held frame, and `fps` fills those slots with copies of it.
    Output time is therefore a frame count, and `at()` counts frames the same way, so a
    line placed with it starts on the frame it was written against — through every cut and
    every hold. The recordings are 60 fps with a dropped frame here and there (eleven in the
    09-23 assembly), and each drop moves everything after it one frame earlier once the output
    is renumbered at an exact 60 — the old seconds-based shift drifted 0.18 s by the end;
    counting frames cannot drift.

    Assembly time t is content time: the frame showing it has pts t + p0, because the concat
    copy starts the first frame at p0 (0.046 s). ffmpeg subtracts p0 before the filters see a
    frame, so in `select` the same frame has t = content time. Keep-range edges are moved to
    the midpoint between two frames, so `select` and this class can never disagree about a
    frame that sits exactly on an edge — and `build` checks the frame count anyway.
    """
    FPS = 60

    def __init__(self, cuts, holds, times=None):
        times = frame_times() if times is None else times
        self.p0 = times[0]
        rel = [t - self.p0 for t in times]

        def snap(x):
            i = bisect.bisect_left(rel, x)
            if i <= 0:
                return rel[0] - 0.5 / self.FPS
            if i >= len(rel):
                return rel[-1] + 0.5 / self.FPS
            return (rel[i - 1] + rel[i]) / 2

        self.keeps = [(snap(s), snap(e)) for s, e in keep_ranges(cuts, rel[-1] + 1.0 / self.FPS)]
        self.sel = [t for t, r in zip(times, rel) if self.kept(r)]
        self.holds = []
        for h, secs in holds:
            if not self.kept(h):
                fail(f"a hold at {h:.2f}s falls inside a cut")
            k = bisect.bisect_right(self.sel, h + self.p0) - 1
            self.holds.append((k, int(round(secs * self.FPS))))
        self.holds.sort()

    def kept(self, t):
        return any(s <= t < e for s, e in self.keeps)

    def frames(self):
        return len(self.sel) + sum(n for _, n in self.holds)

    def length(self):
        return self.frames() / self.FPS

    def at(self, t):
        """Output seconds at which assembly moment t is on screen."""
        pts = t + self.p0
        i = bisect.bisect_right(self.sel, pts) - 1
        if i < 0:
            return 0.0
        if self.kept(t):
            frac = min(max(pts - self.sel[i], 0.0), 1.0 / self.FPS)
        else:                      # inside a cut: it happens when the picture resumes
            i, frac = min(i + 1, len(self.sel) - 1), 0.0
        return (i + sum(n for k, n in self.holds if k < i)) / self.FPS + frac

    def filters(self):
        # ffmpeg hands the filter graph timestamps minus the file's start time, so a filter's
        # `t` is already content time — the first frame is t=0, not t=p0.
        sel = "+".join(f"gte(t,{s:.6f})*lt(t,{e:.6f})" for s, e in self.keeps)
        gaps = "".join(f"+gt(N,{k})*{n}" for k, n in self.holds)
        return f"select='{sel}',setpts='(N{gaps})/({self.FPS}*TB)',fps={self.FPS}"


def build(cuts, out):
    """The picture through `Edit`, the narration placed on Edit's clock, one normalised mix."""
    holds = hold_points()
    edit = Edit(cuts, holds)
    starts = line_starts()
    if cuts or holds:
        video = os.path.join(RUN, "tight-video.mp4" if cuts else "held-video.mp4")
        ff(["-i", ASSEMBLY, "-vf", edit.filters(), "-an", "-c:v", "libx264",
            "-preset", "medium", "-crf", "21", "-pix_fmt", "yuv420p", video])
        got, want = frame_count(video), edit.frames()
        if got != want:
            fail(f"{video} has {got} frames, the edit expects {want}: the filter and the "
                 "clock disagree, so no line could be trusted")
    else:
        video = ASSEMBLY
    args, filters, labels = [], [], []
    for n, (b, i, at, w) in enumerate(starts):
        args += ["-i", w]
        filters.append(f"[{n}:a]aresample={RATE},aformat=channel_layouts=stereo,"
                       f"adelay={int(round(edit.at(at) * int(RATE)))}S:all=1[d{n}]")
        labels.append(f"[d{n}]")
    # normalize=0 keeps a clip at its own level instead of dividing by input count
    chain = ";".join(filters) + (
        f";{''.join(labels)}amix=inputs={len(starts)}:normalize=0:dropout_transition=0[m];"
        f"[m]apad,atrim=0:{edit.length():.6f}[out]")
    ff([*args, "-filter_complex", chain, "-map", "[out]",
        "-c:a", "pcm_s16le", "-ar", RATE, "-ac", CHANNELS, BED])
    normed = os.path.join(RUN, "narration-normed.wav")
    loudnorm(BED, normed)
    ff(["-i", video, "-i", normed, "-map", "0:v", "-map", "1:a", "-c:v", "copy",
        "-c:a", "aac", "-b:a", "192k", "-ar", RATE, "-ac", CHANNELS,
        "-movflags", "+faststart", "-shortest", out])
    held = sum(s for _, s in holds)
    print(f"wrote {out} ({duration(out):.3f}s): {len(starts)} lines, {len(holds)} holds "
          f"({held:.1f}s), {len(cuts)} cuts")
    return out


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
    """The whole assembly, uncut, with the narration and the holds — a debugging aid."""
    require_assembly()
    return build([], OUT)


def edit_for(path):
    """The Edit that produced `path`: the tightened deliverable carries cuts.json's cuts."""
    tight = os.path.abspath(path) == os.path.abspath(SHOWCASE)
    cuts = json.load(open(CUTS)) if tight and os.path.isfile(CUTS) else []
    return Edit(cuts, hold_points())


# ---------------------------------------------------------------- check

def check(path=OUT):
    if not os.path.isfile(path):
        fail(f"no {path}")
    a = anchors()
    edit = edit_for(path)
    shift = edit.at
    out = subprocess.run(
        ["ffmpeg", "-hide_banner", "-nostats", "-i", path, "-map", "0:a",
         "-af", "silencedetect=noise=-45dB:d=0.6", "-f", "null", "-"],
        capture_output=True, text=True).stderr
    ends = [float(l.split("silence_end:")[1].split()[0])
            for l in out.splitlines() if "silence_end:" in l]
    starts = [float(l.split("silence_start:")[1].split()[0])
              for l in out.splitlines() if "silence_start:" in l]

    print("\nsync — first cue audio at its expected post-cut time:")
    silent_cues = 0
    for b in BEATS:
        first_cue = next((cue for cue, _ in narration.cues(b) if cue is not None), 0.0)
        want = shift(a[b]["offset"] + a[b]["action"] - LEAD + first_cue)
        # listen a quarter second in: a clip may open with ~0.1 s of breath, and testing its
        # very first sample reported beat 8 silent while its voice was 20 ms away
        probe = want + 0.25
        silent_at_want = any(s + 0.1 < probe < e - 0.1
                             for s, e in zip(starts, ends + [duration(path)]))
        state = "SILENCE" if silent_at_want else "audible"
        silent_cues += int(silent_at_want)
        print(f"  beat {b}  cue at {want:7.2f}s  {state}")
    print(f"  {len(BEATS) - silent_cues}/{len(BEATS)} cue points have audio")

    print("\ndensity — speech vs picture per beat:")
    for b in BEATS:
        lo = shift(a[b]["offset"])
        hi = shift(a[b]["offset"] + a[b]["video"] - 0.001)
        quiet = 0.0
        for s, e in zip(starts, ends + [duration(path)]):
            if e <= lo or s >= hi:
                continue
            quiet += min(e, hi) - max(s, lo)
        d = 100.0 * (1 - quiet / (hi - lo))
        print(f"  beat {b}  {d:5.1f}%")

    r = subprocess.run(["ffmpeg", "-hide_banner", "-nostats", "-i", path,
                        "-af", "ebur128=peak=true", "-f", "null", "-"],
                       capture_output=True, text=True).stderr.splitlines()
    tail = "\n".join(r[-14:])
    print("\nloudness (EBU R128):")
    for line in tail.splitlines():
        if any(k in line for k in ("I:", "LRA:", "Peak:")):
            print("  " + line.strip())


# ---------------------------------------------------------------- the typed-keys strip

# Below the keymap the HUD prints the keys as they are pressed. That strip is the only
# ground truth fine enough to cue narration that names individual keystrokes: the banner
# says "normal" for twenty seconds straight while the segment types a whole tour, so
# aligning to the banner alone puts a line about yanking over a line about undo.
#
# There is no OCR here — `dub.py keys <N>` stacks samples of the strip into one image for
# a person (or a model) to read, and the timeline goes into SCRIPT.md as a comment above
# the beat's cues.
KEYS_CROP = "crop=740:64:1806:582"


def keys_strip(beat, every=4.0, out=None):
    """Stack samples of the typed-keys strip through a take into one tall image.

    Sampled with the fps filter in a single decode, never with `-ss`: these recordings
    have sparse keyframes and a nominal 60 fps that is really 59.99, so seeking returns a
    frame from several seconds away without saying so. That is not a theoretical risk —
    it put beat 4's cues eight seconds out and then made the check that should have caught
    it agree with them.
    """
    mp4 = os.path.join(RUN, f"take{beat}.mp4")
    if not os.path.isfile(mp4):
        fail(f"missing {mp4}")
    outdir = os.path.join(RUN, f".keys-{beat}")
    shutil.rmtree(outdir, ignore_errors=True)
    os.makedirs(outdir, exist_ok=True)
    ff(["-i", mp4, "-vf", f"{KEYS_CROP},fps=1/{every:g}", "-f", "image2",
        os.path.join(outdir, "%03d.png")])
    frames = sorted(f for f in os.listdir(outdir) if f.endswith(".png"))
    out = out or os.path.join(RUN, f"keys-take{beat}.png")
    args, filt = [], ""
    for n, f in enumerate(frames):
        args += ["-i", os.path.join(outdir, f)]
        filt += f"[{n}:v]"
    ff([*args, "-filter_complex", f"{filt}vstack=inputs={len(frames)}[o]", "-map", "[o]", out])
    shutil.rmtree(outdir, ignore_errors=True)
    print(f"wrote {out} — {len(frames)} rows, row n is t={every:g}*(n-1)s")
    return out


def frange(a, b, step):
    t = a
    while t < b:
        yield t
        t += step


# ---------------------------------------------------------------- tighten

SHOWCASE = os.path.join(RUN, "showcase.mp4")      # the deliverable
CUTS = os.path.join(RUN, "cuts.json")

STILL = float(os.environ.get("ZMK_DUB_STILL", "0.6"))    # mean |Δgrey| below this = frozen
MINGAP = float(os.environ.get("ZMK_DUB_MINGAP", "0.9"))  # shorter than this is rhythm
KEEPGAP = float(os.environ.get("ZMK_DUB_KEEPGAP", "0.3"))  # leave this much of each pause
PAD_AFTER, PAD_BEFORE = 0.35, 0.25                       # breathing room around a line


def speech_spans():
    """[(start, end)] of every rendered line, in assembly time.

    A line that plays through a hold covers less assembly time than its own length — the
    held part is spoken over one frame — so its end is found through the holds.
    """
    holds = hold_points()
    return sorted((at, contract(expand(at, holds) + duration(w), holds))
                  for _, _, at, w in line_starts())


def motion(beat, fps=8.0, w=48, h=36):
    """Mean |Δgrey| between consecutive samples of the content pane."""
    raw = subprocess.run(
        ["ffmpeg", "-v", "error", "-i", os.path.join(RUN, f"take{beat}.mp4"),
         "-vf", f"crop=1790:1330:10:60,fps={fps:g},scale={w}:{h}",
         "-f", "rawvideo", "-pix_fmt", "gray", "-"], capture_output=True).stdout
    sz, n = w * h, len(raw) // (w * h)
    return fps, [sum(abs(raw[(i - 1) * sz + j] - raw[i * sz + j]) for j in range(sz)) / sz
                 for i in range(1, n)]


def frame_bytes(beat, t_local, w=64, h=36):
    """One frame of a take, by frame index.

    Not `-ss`: these files seek badly (see keys_strip), and near the end of a take an
    input seek returns nothing at all — which used to make find_pauses silently drop the
    cut, losing every take's dead tail without a word.
    """
    mp4 = os.path.join(RUN, f"take{beat}.mp4")
    n = max(0, int(round(t_local * 60)))
    # the same region motion() measures. Comparing whole frames instead rejects cuts whose
    # content is frozen, because the HUD's typed-keys strip decays and its clock ticks —
    # changes that are not the picture the viewer is watching.
    return subprocess.run(
        ["ffmpeg", "-v", "error", "-i", mp4, "-vf",
         f"select='eq(n\\,{n})',crop=1790:1330:10:60,scale={w}:{h}", "-frames:v", "1",
         "-fps_mode", "passthrough", "-f", "rawvideo", "-pix_fmt", "gray", "-"],
        capture_output=True).stdout


def find_pauses(verify=True):
    """Ranges that are both silent and visually frozen — the ones safe to remove.

    Silence alone is not enough: a pause while the segment is typing is the demo, not
    dead air. A cut is only proposed where nothing is being said AND the picture is not
    moving, and `verify` then re-checks each candidate by comparing the frame at its
    start with the frame at its end — if they differ, something happened in there after
    all and the cut is dropped.
    """
    a, spans, cuts = anchors(), speech_spans(), []

    def talking(t):
        return any(s - PAD_BEFORE <= t <= e + PAD_AFTER for s, e in spans)

    for b in BEATS:
        fps, m = motion(b)
        off, vid, cs = a[b]["offset"], a[b]["video"], a[b]["content_start"]
        run = None
        for i, val in enumerate(m):
            t = (i + 1) / fps
            # inside the head the rail being drawn is a motion spike, but it is not this
            # take's picture starting — do not let it break the run in two
            still = val < STILL or t < cs - 0.2
            dead = still and not talking(off + t)
            if dead and run is None:
                run = t
            elif not dead and run is not None:
                if t - run >= MINGAP:
                    cuts.append((b, off + run + KEEPGAP / 2, off + t - KEEPGAP / 2))
                run = None
        if run is not None and vid - run >= MINGAP:
            cuts.append((b, off + run + KEEPGAP / 2, off + vid - 0.05))

    # KEEPGAP leaves a little of every pause so a cut does not feel abrupt — but nothing
    # precedes the video's first frame, and keeping it opened the video on a bare desktop
    # for a third of a second before the HUD appeared.
    if cuts and cuts[0][0] == BEATS[0] and cuts[0][1] <= 0.5:
        cuts[0] = (cuts[0][0], 0.0, cuts[0][2])

    if verify:
        kept = []
        for b, s, e in cuts:
            off, cs = a[b]["offset"], a[b]["content_start"]
            # Before content_start the screen still belongs to the previous beat, so there
            # is nothing of this take's to protect and the frame check does not apply. It
            # would reject the cut anyway: the HUD rail is drawn partway through, so the
            # first and last frames differ for a reason that is setup, not content.
            if e <= off + cs + 0.05:
                kept.append((b, s, e))
                continue
            # sample just inside the window: the very last frame of a take may not
            # decode, and a silent drop here costs a whole dead tail
            f0 = frame_bytes(b, s - off + 0.1)
            f1 = frame_bytes(b, min(e - off - 0.1, a[b]["video"] - 0.2))
            if not f0 or not f1 or len(f0) != len(f1):
                print(f"  beat {b} {s:.2f}→{e:.2f}: could not sample, keeping the cut")
                kept.append((b, s, e))
                continue
            d = sum(abs(f0[i] - f1[i]) for i in range(len(f0))) / len(f0)
            if d <= 3.0:
                kept.append((b, s, e))
            else:
                print(f"  dropped beat {b} {s:.2f}→{e:.2f} ({e-s:.2f}s): "
                      f"picture moves across it (diff {d:.1f})")
        cuts = kept

    # a cut must never swallow a held frame
    holds, safe = hold_points(), []
    for b, s, e in cuts:
        inside = [h for h, _ in holds if s <= h < e]
        if inside:
            print(f"  dropped beat {b} {s:.2f}→{e:.2f}: it would cut the frame held at {inside[0]:.2f}")
        else:
            safe.append((b, s, e))
    cuts = safe

    json.dump([[b, round(s, 3), round(e, 3)] for b, s, e in cuts],
              open(CUTS, "w"), indent=1)
    return cuts


def keep_ranges(cuts, total):
    out, at = [], 0.0
    for _, s, e in sorted(cuts, key=lambda c: c[1]):
        if s > at:
            out.append((at, s))
        at = max(at, e)
    if at < total:
        out.append((at, total))
    return out


def tighten():
    require_assembly()
    cuts = find_pauses()
    total = duration(ASSEMBLY)
    removed = sum(e - s for _, s, e in cuts)
    held = sum(s for _, s in hold_points())
    print(f"{len(cuts)} cuts, {removed:.1f}s removed, {held:.1f}s held, "
          f"{total:.1f}s → {total - removed + held:.1f}s")
    return build(cuts, SHOWCASE)


# ---------------------------------------------------------------- proof

def proof(beat, video=None, out=None):
    """One panel per narration line: the screen above the typed-keys strip, at the moment
    that line is being spoken, taken from the finished video.

    Every frame comes out of a single decode driven by `select`, never by `-ss`, for the
    reason in keys_strip's docstring. Read the strip under each panel and check it against
    the words — that is the only check that actually caught beat 4, after two rounds of
    weaker ones passed while the cues were eight seconds out.
    """
    video = video or SHOWCASE
    if not os.path.isfile(video):
        fail(f"missing {video}")
    a = anchors()
    place = edit_for(video).at

    base = a[beat]["offset"] + a[beat]["action"] - LEAD
    marks, texts = [], []
    for i, (cue, text) in enumerate(narration.cues(beat)):
        w = os.path.join(CLIPS, f"beat{beat}-{i:02d}.wav")
        if not os.path.isfile(w):
            continue
        marks.append(place(max(0.0, base + (cue or 0.0))) + duration(w) * 0.55)
        texts.append(text)
    if not marks:
        fail(f"beat {beat}: nothing rendered")

    outdir = os.path.join(RUN, f".proof-{beat}")
    shutil.rmtree(outdir, ignore_errors=True)
    os.makedirs(outdir, exist_ok=True)
    # exact frame indices, not a time window: `between(t,m,m+0.05)` catches three frames
    # at 60 fps, so the panels silently come from the first few cues repeated
    rate = subprocess.run(
        ["ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries",
         "stream=r_frame_rate", "-of", "csv=p=0", video],
        capture_output=True, text=True).stdout.strip()
    num, _, den = rate.partition("/")
    fps = float(num) / float(den or 1)
    sel = "+".join(f"eq(n\\,{round(m * fps)})" for m in marks)
    # split first: a filter graph cannot read [0:v] twice
    ff(["-i", video, "-filter_complex",
        f"[0:v]select='{sel}',split=2[p][q];"
        f"[p]crop=1790:1010:10:60,scale=700:-2[a];"
        f"[q]{KEYS_CROP},scale=700:-2[b];[a][b]vstack=inputs=2[o]",
        "-map", "[o]", "-fps_mode", "passthrough", os.path.join(outdir, "%03d.png")])

    frames = sorted(f for f in os.listdir(outdir) if f.endswith(".png"))[:len(marks)]
    out = out or os.path.join(RUN, f"proof-beat{beat}.png")
    args, filt = [], ""
    for n, f in enumerate(frames):
        args += ["-i", os.path.join(outdir, f)]
        filt += f"[{n}:v]"
    ff([*args, "-filter_complex", f"{filt}hstack=inputs={len(frames)}[o]", "-map", "[o]", out])
    shutil.rmtree(outdir, ignore_errors=True)
    print(f"wrote {out} — {len(frames)} panels, left to right:")
    for m, t in zip(marks, texts):
        print(f"  {m:7.1f}  {t[:72]}")
    return out


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
        act = a[b]["action"]
        hs = [(act + c, s) for c, s in narration.holds(b)]          # take seconds
        tl = hud_timeline(b)
        drawn = [e for _, e, st in tl if st != "none"]
        hud_end = max(drawn) if drawn else a[b]["video"]
        starts = [act - LEAD + (cue or 0.0) for cue, _ in cs]     # take seconds
        spoken = 0.0
        held = ", ".join(f"+{c:.1f} for {s:.1f}s" for c, s in narration.holds(b))
        print(f"\nbeat {b} — take {a[b]['video']:.1f}s, action at {act:.1f}s, HUD until "
              f"+{hud_end - act:.1f}" + (f", holds {held}" if held else ""))
        for i, (cue, text) in enumerate(cs):
            wav = os.path.join(CLIPS, f"beat{b}-{i:02d}.wav")
            dur = clip_seconds(text, wav)
            spoken += dur
            # compare lines on the clock the viewer hears them on: holds played out
            s0 = expand(starts[i], hs)
            e0 = s0 + dur
            end = contract(e0, hs)                                  # back to take seconds
            note = ""
            if i + 1 < len(cs) and e0 > expand(starts[i + 1], hs) - 0.05:
                note, bad = f"  OVERLAPS next by {e0 - expand(starts[i + 1], hs):.1f}s", bad + 1
            elif end > hud_end - 0.05:
                note, bad = f"  RUNS PAST the HUD by {end - hud_end:.1f}s", bad + 1
            src = "meas" if os.path.isfile(wav) else "est"
            # the layer the HUD is actually showing while this line is spoken
            st0, st1 = hud_at(tl, starts[i]), hud_at(tl, end)
            span = st0 if st0 == st1 else f"{st0}→{st1}"
            print(f"  +{cue or 0.0:5.1f} → {end - act + LEAD:5.1f}  {dur:4.1f}s ({src}, "
                  f"{len(text.split()):3d}w)  HUD {span:<16}{note}")
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
    if cmd == "assemble":
        assemble()
    elif cmd == "anchors":
        for b, d in measure_anchors().items():
            print(f"take{b}  action={d['action']:>7.3f}  video={d['video']:>7.3f}  "
                  f"offset={d['offset']:>8.3f}")
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
    elif cmd == "keys":
        every = float(sys.argv[3]) if len(sys.argv) > 3 else 6.0
        for b in (sys.argv[2:3] or BEATS):
            keys_strip(b, every)
    elif cmd == "proof":
        proof(sys.argv[2], sys.argv[3] if len(sys.argv) > 3 else None)
    elif cmd == "pauses":
        for b, s_, e_ in find_pauses():
            print(f"  beat {b}  {s_:8.2f} → {e_:8.2f}  {e_-s_:5.2f}s")
    elif cmd == "tighten":
        tighten()
    elif cmd == "check":
        check(sys.argv[2] if len(sys.argv) > 2 else OUT)
    else:
        print(USAGE)
        sys.exit(2)


if __name__ == "__main__":
    main()
