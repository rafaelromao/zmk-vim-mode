"""Tests for dub.py's edit clock and narration.py's holds: no ffmpeg, no takes needed.

    python3 -m unittest showcase.dub_test
"""
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import dub  # noqa: E402
import narration  # noqa: E402

P0 = 0.046419          # the assembly's first frame is not at zero
FPS_SRC = 59.99        # and the recordings are not quite 60 fps


def times(seconds=10.0):
    return [P0 + i / FPS_SRC for i in range(int(seconds * FPS_SRC))]


class ExpandContract(unittest.TestCase):
    H = [(5.0, 1.0), (8.0, 2.0)]

    def test_round_trip_outside_holds(self):
        for t in (0.0, 4.9, 5.0, 5.2, 7.9, 8.5, 9.0):
            self.assertAlmostEqual(dub.contract(dub.expand(t, self.H), self.H), t)

    def test_inside_a_hold_maps_to_the_held_frame(self):
        self.assertEqual(dub.contract(5.5, self.H), 5.0)
        self.assertEqual(dub.contract(9.5, self.H), 8.0)   # 8.0 + the earlier 1.0 s hold

    def test_later_holds_add_up(self):
        self.assertAlmostEqual(dub.expand(9.0, self.H), 12.0)


class EditClock(unittest.TestCase):
    def test_counts_frames_not_seconds(self):
        e = dub.Edit([], [], times=times())
        # frame 598 of a 59.99 fps source is at 9.9683 s of content, but it is output
        # frame 598 of a 60 fps video: 9.9667 s. The clock must say the latter.
        t = e.sel[598] - P0
        self.assertAlmostEqual(e.at(t), 598 / 60, places=6)

    def test_cut_removes_its_frames(self):
        e = dub.Edit([("x", 2.0, 3.0)], [], times=times())
        before = sum(1 for t in times() if t - P0 < 2.0)
        self.assertAlmostEqual(e.at(3.0), before / 60, delta=1 / 60)
        self.assertEqual(e.at(2.5), e.at(3.0))                 # inside a cut: resumes after it

    def test_hold_delays_only_what_follows(self):
        e = dub.Edit([], [(5.0, 1.0)], times=times())
        self.assertEqual(e.frames(), len(times()) + 60)
        self.assertLess(e.at(4.99), 5.0)                        # before the held frame: unchanged
        self.assertAlmostEqual(e.at(5.05) - e.at(4.99), 1.0 + 0.06, delta=1 / 60)

    def test_filter_holds_the_frame_the_clock_holds(self):
        e = dub.Edit([], [(5.0, 1.0)], times=times())
        k, n = e.holds[0]
        self.assertIn(f"gt(N,{k})*{n}", e.filters())
        self.assertTrue(e.filters().endswith(",fps=60"))

    def test_select_is_in_content_time(self):
        # ffmpeg subtracts the file's start time before the filters see a frame: `select`
        # must compare against content time, not pts. Adding p0 back shifted every cut edge
        # by three frames in the first build and only the frame-count check caught it.
        e = dub.Edit([("x", 2.0, 3.0)], [], times=times())
        s, _ = e.keeps[1]
        self.assertIn(f"gte(t,{s:.6f})", e.filters())
        self.assertNotIn(f"gte(t,{s + P0:.6f})", e.filters())

    def test_a_dropped_frame_moves_what_follows_one_frame_earlier(self):
        ts = [P0 + i / 60 for i in range(300)]
        del ts[100]                                            # the recorder dropped one
        e = dub.Edit([], [], times=ts)
        # ts[200] is the frame recorded at 201/60 s; renumbered at 60 fps it plays at 200/60
        self.assertAlmostEqual(ts[200] - P0, 201 / 60, places=9)
        self.assertAlmostEqual(e.at(ts[200] - P0), 200 / 60, places=6)

    def test_hold_inside_a_cut_is_refused(self):
        with self.assertRaises(SystemExit):
            dub.Edit([("x", 4.0, 6.0)], [(5.0, 1.0)], times=times())


class Holds(unittest.TestCase):
    def test_parsed_outside_the_quote_and_never_spoken(self):
        with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False) as f:
            f.write("### 3 · Beat\n\n**Hold** +16.3 for 5.5 s — a still frame\n\n"
                    "**Narration.**\n> [+5.0] First line.\n> [+20.0] Second.\n\n### 4 · Next\n"
                    "**Hold** +1.0 for 2.0 s — another beat's\n")
            path = f.name
        try:
            self.assertEqual(narration.holds("3", path), [(16.3, 5.5)])
            self.assertEqual([t for _, t in narration.cues("3", path)], ["First line.", "Second."])
        finally:
            os.remove(path)


if __name__ == "__main__":
    unittest.main()
