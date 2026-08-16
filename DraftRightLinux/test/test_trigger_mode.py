"""TriggerMode enum + its settings persistence (#188). GTK-free.

Runnable without a display / GTK:  python3 test/test_trigger_mode.py
"""

import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from draftright.models.trigger_mode import TriggerMode
from draftright.services.settings_service import SettingsService


class TriggerModeTest(unittest.TestCase):
    def test_mutually_exclusive_flags(self):
        self.assertTrue(TriggerMode.PENCIL.uses_pencil)
        self.assertFalse(TriggerMode.PENCIL.uses_hotkey)
        self.assertFalse(TriggerMode.HOTKEY.uses_pencil)
        self.assertTrue(TriggerMode.HOTKEY.uses_hotkey)

    def test_wire_values_are_cross_platform_stable(self):
        # Must match macOS/Windows TriggerMode raw values, or settings.json
        # drifts when synced between platforms.
        self.assertEqual(TriggerMode.PENCIL.value, "pencil")
        self.assertEqual(TriggerMode.HOTKEY.value, "hotkey")

    def test_from_wire_parses_and_defaults_to_hotkey(self):
        self.assertIs(TriggerMode.from_wire("pencil"), TriggerMode.PENCIL)
        self.assertIs(TriggerMode.from_wire("hotkey"), TriggerMode.HOTKEY)
        self.assertIs(TriggerMode.from_wire("both"), TriggerMode.HOTKEY)  # removed elsewhere
        self.assertIs(TriggerMode.from_wire(None), TriggerMode.HOTKEY)
        self.assertIs(TriggerMode.from_wire("garbage"), TriggerMode.HOTKEY)


class TriggerModeSettingsTest(unittest.TestCase):
    def test_default_is_hotkey(self):
        # No stored value → Hotkey. Existing users (hotkey-only until now) are
        # unchanged on upgrade, and Hotkey works on both X11 and Wayland.
        s = SettingsService()
        s._data.pop("trigger_mode", None)
        self.assertIs(s.trigger_mode, TriggerMode.HOTKEY)

    def test_round_trips(self):
        s = SettingsService()
        s.trigger_mode = TriggerMode.PENCIL
        self.assertIs(s.trigger_mode, TriggerMode.PENCIL)
        s.trigger_mode = TriggerMode.HOTKEY
        self.assertIs(s.trigger_mode, TriggerMode.HOTKEY)


if __name__ == "__main__":
    unittest.main()
