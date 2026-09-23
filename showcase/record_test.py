#!/usr/bin/env python3
"""Tests for the temporary HUD timing used by screen captures."""

import tempfile
import unittest
from pathlib import Path

from showcase.record import CAPTURE_PRESS_MS, restore_hud_config, set_capture_press_ms


class CapturePressMsTest(unittest.TestCase):
    def test_capture_override_is_restored_without_changing_other_settings(self):
        original = (
            "hud:\n"
            "  press_ms: 500               # key flash\n"
            "  release_ms: 60              # release fade\n"
            "feed:\n"
            "  dead_key_ms: 35\n"
        )
        with tempfile.TemporaryDirectory() as directory:
            config = Path(directory) / "config.yaml"
            config.write_text(original, encoding="utf-8")
            saved = set_capture_press_ms(config)
            try:
                current = config.read_text(encoding="utf-8")
                self.assertIn(f"press_ms: {CAPTURE_PRESS_MS}", current)
                self.assertIn("release_ms: 60", current)
                self.assertIn("dead_key_ms: 35", current)
            finally:
                restore_hud_config(saved)
            self.assertEqual(config.read_text(encoding="utf-8"), original)


if __name__ == "__main__":
    unittest.main()
