#!/usr/bin/env python3
"""Focused tests for the Diamond typist model in rehearsal-feed.py."""

import importlib.util
import sys
import unittest
from pathlib import Path

SHOW = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("rehearsal_feed", SHOW / "rehearsal-feed.py")
feed = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = feed
spec.loader.exec_module(feed)


class InjectedKeysTest(unittest.TestCase):
    def setUp(self):
        self.out = []
        self.layers = []
        self.keymap = {
            "positions": {"33": 22, "34": 23},
            "zmk_layers": {
                "0": {"id": 0, "name": "OMARCHY", "drawer": "alpha1"},
                "2": {"id": 2, "name": "NORMAL", "drawer": "vim"},
                "13": {"id": 13, "name": "ALPHA2", "drawer": "alpha2"},
                "14": {"id": 14, "name": "NUMBERS", "drawer": "numbers"},
                "18": {"id": 18, "name": "SHIFTED ALPHA2", "drawer": "shifted2"},
            },
        }
        self.injected = feed.InjectedKeys(
            self.out.append,
            lambda *_: None,
            keymap=lambda: self.keymap,
            layers=lambda: list(self.layers),
        )

    def send(self, code, value):
        self.out.extend(self.injected.messages(code, value))

    def type_text(self, text):
        for char in text:
            if char == " ":
                code = feed.E.KEY_SPACE
            else:
                code = next(code for code, chars in feed.CHARS.items() if chars[0] == char)
            self.send(code, 1)
            self.send(code, 0)

    def test_alpha2_and_magic_sequence(self):
        self.type_text("video ever have the kx")

        downs = [
            (i, msg) for i, msg in enumerate(self.out)
            if msg.get("kind") == "key" and msg.get("type") == "keyDown"
        ]
        self.assertEqual("".join(msg["chars"] for _, msg in downs), "video ever have the kx")

        # Word-start v and the final k/x are alpha2 taps. The v after the e in ever,
        # and both magic keys in have, stay on alpha1.
        alpha2_downs = [(i, msg["chars"]) for i, msg in downs if msg["chars"] in {"v", "k", "x"}
                        and self.out[i - 1] == {"kind": "layers", "ids": [13]}]
        self.assertEqual([char for _, char in alpha2_downs], ["v", "k", "x"])
        for i, char in alpha2_downs:
            self.assertEqual(self.out[i + 1:i + 3],
                             [{"kind": "press", "pos": 33}, {"kind": "release", "pos": 33}])
            self.assertEqual(self.out[i + 3]["type"], "keyUp")
            self.assertEqual(self.out[i + 4], {"kind": "layers", "ids": []})

        self.assertFalse(any(msg.get("pill") or msg.get("combo") for msg in self.out))

    def test_numbers_restore_the_reported_stack(self):
        self.layers[:] = [2]
        self.send(feed.E.KEY_1, 1)
        self.assertEqual(self.out[0], {"kind": "layers", "ids": [2, 14]})
        self.send(feed.E.KEY_1, 0)
        self.assertEqual(self.out[-1], {"kind": "layers", "ids": [2]})

    def test_uppercase_alpha2_flashes_both_thumbs(self):
        self.send(feed.E.KEY_LEFTSHIFT, 1)
        self.send(feed.E.KEY_K, 1)
        self.assertEqual(self.out[1], {"kind": "layers", "ids": [18]})
        self.assertEqual(self.out[3:7], [
            {"kind": "press", "pos": 33},
            {"kind": "release", "pos": 33},
            {"kind": "press", "pos": 34},
            {"kind": "release", "pos": 34},
        ])

    def test_vim_command_letters_do_not_enter_alpha2(self):
        self.layers[:] = [2]
        self.send(feed.E.KEY_K, 1)
        self.assertEqual([msg["kind"] for msg in self.out], ["key"])


if __name__ == "__main__":
    unittest.main()
