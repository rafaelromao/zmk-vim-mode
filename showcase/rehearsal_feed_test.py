#!/usr/bin/env python3
"""Focused tests for the Diamond typist model in rehearsal-feed.py."""

import importlib.util
import sys
import unittest
from pathlib import Path

SHOW = Path(__file__).resolve().parent
sys.path.insert(0, str(SHOW))
spec = importlib.util.spec_from_file_location("rehearsal_feed", SHOW / "rehearsal-feed.py")
feed = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = feed
spec.loader.exec_module(feed)


class InjectedKeysTest(unittest.TestCase):
    def setUp(self):
        self.out = []
        # ZMK's active stack includes the Alpha 1 base layer. The HUD omits id 0
        # when rendering, but a one-shot layer restore must preserve that base id.
        self.layers = [0]
        self.keymap = {
            "positions": {"32": 21, "33": 22, "34": 23},
            "zmk_layers": {
                "0": {"id": 0, "name": "OMARCHY", "drawer": "alpha1"},
                "2": {"id": 2, "name": "NORMAL", "drawer": "vim"},
                "7": {"id": 7, "name": "CMDLINE", "drawer": "vim"},
                "8": {"id": 8, "name": "INSERT", "drawer": "vim"},
                "13": {"id": 13, "name": "ALPHA2", "drawer": "alpha2"},
                "14": {"id": 14, "name": "NUMBERS", "drawer": "numbers"},
                "15": {"id": 15, "name": "SYMBOLS", "drawer": "symbols"},
                "18": {"id": 18, "name": "SHIFTED ALPHA2", "drawer": "shifted2"},
            },
            "layers": {
                "vim": [{"tap": char, "type": "key"}
                        for char in "bewgouqnsrhjklfctia."],
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
                unshifted = next((code for code, chars in feed.CHARS.items()
                                  if chars[0] == char), None)
                if unshifted is not None:
                    code = unshifted
                    shifted = False
                else:
                    code = next(code for code, chars in feed.CHARS.items()
                                if chars[1] == char)
                    shifted = True
            if char == " ":
                shifted = False
            if shifted:
                self.send(feed.E.KEY_LEFTSHIFT, 1)
            self.send(code, 1)
            self.send(code, 0)
            if shifted:
                self.send(feed.E.KEY_LEFTSHIFT, 0)

    def assert_layered_character(self, char, layer_id, activator_pos, restore):
        down = next(i for i, msg in enumerate(self.out)
                    if msg.get("kind") == "key" and msg.get("type") == "keyDown"
                    and msg.get("chars") == char)
        self.assertEqual(self.out[down - 1], {"kind": "layers", "ids": layer_id})
        self.assertEqual(self.out[down + 1:down + 3], [
            {"kind": "press", "pos": activator_pos},
            {"kind": "release", "pos": activator_pos},
        ])
        up = next(i for i in range(down + 1, len(self.out))
                  if self.out[i].get("kind") == "key" and self.out[i].get("type") == "keyUp"
                  and self.out[i].get("chars") == char)
        self.assertEqual(self.out[up + 1], {"kind": "layers", "ids": restore})

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
                        and self.out[i - 1] == {"kind": "layers", "ids": [0, 13]}]
        self.assertEqual([char for _, char in alpha2_downs], ["v", "k", "x"])
        for i, char in alpha2_downs:
            self.assertEqual(self.out[i + 1:i + 3],
                             [{"kind": "press", "pos": 33}, {"kind": "release", "pos": 33}])
            self.assertEqual(self.out[i + 3]["type"], "keyUp")
            self.assertEqual(self.out[i + 4], {"kind": "layers", "ids": [0]})

    def test_digits_use_numbers_layer_instead_of_combo(self):
        self.layers[:] = [2]
        self.type_text("0125")
        for char in "0125":
            self.assert_layered_character(char, [2, 14], 32, [2])

    def test_symbols_use_symbols_or_numbers_layer_instead_of_combos(self):
        self.layers[:] = [0]
        self.type_text("! - / : |")
        for char in "!-/:":
            self.assert_layered_character(char, [0, 15], 33, [0])
        self.assert_layered_character("|", [0, 14], 32, [0])

        self.out.clear()
        self.type_text(".")
        dot = next(i for i, msg in enumerate(self.out)
                   if msg.get("kind") == "key" and msg.get("type") == "keyDown")
        self.assertEqual(self.out[dot]["chars"], ".")
        self.assertEqual(self.out[0]["kind"], "key")  # Alpha 1's plain dot key, no layer/combo

    def test_cmdline_text_uses_alpha2_while_single_key_normal_commands_stay_on_vim(self):
        self.layers[:] = [0, 7]
        self.type_text("z")
        self.assert_layered_character("z", [0, 7, 13], 33, [0, 7])

        self.out.clear()
        self.layers[:] = [2]
        self.type_text("j")
        down = next(msg for msg in self.out
                    if msg.get("kind") == "key" and msg.get("type") == "keyDown")
        self.assertEqual(down["chars"], "j")
        self.assertNotIn({"kind": "layers", "ids": [2, 13]}, self.out)

    def test_vim_letters_without_single_key_bindings_use_alpha2(self):
        self.layers[:] = [2]
        self.type_text("zy")
        self.assert_layered_character("z", [2, 13], 33, [2])
        self.assert_layered_character("y", [2, 13], 33, [2])

        self.out.clear()
        self.send(feed.E.KEY_ESC, 1)
        self.send(feed.E.KEY_ESC, 0)
        self.type_text("v")
        self.assert_layered_character("v", [2, 13], 33, [2])

    def test_symbol_command_uses_symbols_layer(self):
        self.layers[:] = [2]
        self.type_text(":")
        self.assert_layered_character(":", [2, 15], 33, [2])

    def test_uppercase_alpha2_flashes_both_thumbs(self):
        self.send(feed.E.KEY_LEFTSHIFT, 1)
        self.send(feed.E.KEY_K, 1)
        self.assertEqual(self.out[1], {"kind": "layers", "ids": [0, 18]})
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
