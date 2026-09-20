"""The narration of a beat: the `>` quote lines under its `### N` heading in SCRIPT.md.

Shared by record.py (the Linux scratch track) and dub.py (the deliverable dub), so the
two can never disagree about what a beat says.

A narration line may open with a cue — `[+12.3]` — meaning "start this line 12.3 seconds
after the take's first on-screen action". A line without a cue behaves exactly as it did
before cues existed: it is simply joined onto the run of text before it. A beat whose
lines carry no cues at all therefore renders as one clip, the way every beat used to.

Cues are relative to the action, not to the file, because the takes open with a few
seconds of nothing: record.py starts the recorder before it starts the segment. dub.py
measures that lead-in per take and adds it, so the script never has to know about it.
"""

import os
import re

SHOW = os.path.dirname(os.path.abspath(__file__))
SCRIPT = os.path.join(SHOW, "SCRIPT.md")

CUE = re.compile(r"^\[\+(\d+(?:\.\d+)?)\]\s*")


def _speakable(text):
    """Script punctuation the voice should not try to pronounce."""
    text = text.replace("—", ", ").replace("–", ", ")
    text = re.sub(r"\s{2,}", " ", text)
    text = re.sub(r"\bssh\b", "S S H", text)
    return text.strip()


def raw_lines(beat, script=SCRIPT):
    """The beat's `>` lines, cues intact, one string per line."""
    out, in_beat = [], False
    with open(script) as f:
        for line in f:
            if line.startswith("### "):
                in_beat = line.startswith(f"### {beat} ")
            elif in_beat and line.startswith(">"):
                out.append(line[1:].strip())
    return out


def cues(beat, script=SCRIPT):
    """The beat as [(cue_seconds | None, speakable text), ...].

    Consecutive uncued lines are merged into the line that carries the cue above them,
    so a cue owns everything up to the next cue.
    """
    chunks = []
    for line in raw_lines(beat, script):
        if not line:
            continue
        m = CUE.match(line)
        if m:
            chunks.append([float(m.group(1)), line[m.end():].strip()])
        elif chunks:
            chunks[-1][1] = f"{chunks[-1][1]} {line}".strip()
        else:
            chunks.append([None, line])
    return [(c, _speakable(t)) for c, t in chunks if _speakable(t)]


def flat_text(beat, script=SCRIPT):
    """The whole beat as one string, cues stripped — what record.py's scratch track wants."""
    parts = [t for _, t in cues(beat, script)]
    return _speakable(" ".join(parts)) if parts else ""


def word_count(beat, script=SCRIPT):
    return len(flat_text(beat, script).split())


if __name__ == "__main__":
    import sys
    beats = sys.argv[1:] or [str(n) for n in range(10)]
    for b in beats:
        cs = cues(b)
        print(f"=== beat {b}: {word_count(b)} words, {len(cs)} cue(s) ===")
        for cue, text in cs:
            head = f"[+{cue:>5.1f}]" if cue is not None else "[  --- ]"
            print(f"  {head} {text[:96]}{'…' if len(text) > 96 else ''}")
