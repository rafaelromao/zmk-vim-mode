#!/usr/bin/env python3
"""Romak typist classification and Alpha 2 pacing tests."""

import unittest

from showcase.typist import (ALPHA2_CHARS, NUMBER_LAYER_CHARS, SYMBOL_LAYER_CHARS,
                             type_gap, uses_alpha2)


class TypistTest(unittest.TestCase):
    def test_alpha2_and_magic_key_choices(self):
        self.assertTrue(uses_alpha2("k"))
        self.assertTrue(uses_alpha2("K"))
        self.assertTrue(uses_alpha2("v", "b"))
        self.assertFalse(uses_alpha2("v", "e"))
        self.assertTrue(uses_alpha2("h", "a"))
        self.assertFalse(uses_alpha2("h", "b"))

    def test_alpha2_chars_get_extra_time_but_alpha1_and_vim_commands_do_not(self):
        base, extra = 0.2, 0.1
        self.assertAlmostEqual(type_gap("k", None, base, extra), 0.3)
        self.assertAlmostEqual(type_gap("e", None, base, extra), 0.2)
        self.assertAlmostEqual(type_gap("v", "b", base, extra), 0.3)
        self.assertAlmostEqual(type_gap("v", "e", base, extra), 0.2)
        self.assertAlmostEqual(type_gap("k", None, base, extra, slow_alpha2=False), 0.2)

    def test_literal_glyph_classes_have_explicit_layers(self):
        self.assertTrue(set("0123456789\\{}&()|[]") <= NUMBER_LAYER_CHARS)
        self.assertTrue(set('~#%=:@^$"?-+<>`!/*') <= SYMBOL_LAYER_CHARS)
        self.assertTrue({"'", "_", "z"} <= ALPHA2_CHARS)
        self.assertFalse({"'", "_"} & SYMBOL_LAYER_CHARS)


if __name__ == "__main__":
    unittest.main()
