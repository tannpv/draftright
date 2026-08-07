"""Tray-helper busy pulse (#6) — must run in its own process.

The helper is GTK3 (Ayatana AppIndicator3 has no GTK4 build), and a single
Python process cannot load both GTK 3.0 and 4.0. Importing it alongside the
GTK4 UI tests fails with "Namespace Gtk is already loaded with version 4.0",
which is the very reason the helper is a separate process at runtime.

Runnable without a tray host: python3 test/test_tray_busy.py
"""

import tempfile
import unittest
from unittest import mock

import gi

gi.require_version("Gtk", "3.0")
gi.require_version("AyatanaAppIndicator3", "0.1")
from gi.repository import GLib, Gtk

from draftright import config
from draftright.models.health import HealthStatus
from draftright.tray_helper import TrayHelper


def _run_main_loop(ms: int) -> None:
    """Pump GTK's real main loop for *ms* — the loop the helper actually runs."""
    GLib.timeout_add(ms, lambda: (Gtk.main_quit(), False)[1])
    Gtk.main()


class TrayBusyPulseTest(unittest.TestCase):
    """One-Click has no window, so the tray is the only progress signal."""

    def _helper(self) -> TrayHelper:
        helper = TrayHelper.__new__(TrayHelper)
        helper._status = HealthStatus.CONNECTED
        helper._update_available = False
        helper._busy_source = None
        helper._busy_stop_source = None
        helper._busy_frame = 0
        helper._busy_started_us = 0
        helper._icon = "before"
        helper._icon_dir = tempfile.mkdtemp(prefix=config.TRAY_ICON_NAME_PREFIX)
        helper._indicator = mock.Mock()
        # Menu rows set_status/set_update_available write through to.
        helper._status_item = mock.Mock()
        helper._update_item = mock.Mock()
        return helper

    def _painted(self, helper) -> list:
        return [c.args[0] for c in helper._indicator.set_icon_full.call_args_list]

    # -- the reported bug --------------------------------------------------

    def test_a_fast_rewrite_still_shows_the_pulse(self):
        # A backend cache hit returns in well under one frame. Stopping
        # immediately changed the icon once and changed it straight back —
        # indistinguishable from nothing happening, which is exactly what was
        # reported: "do not see the spin".
        helper = self._helper()
        helper.set_busy(True)
        helper.set_busy(False)                  # finished instantly
        self.assertIsNotNone(helper._busy_source, "the pulse stopped at once")
        self.assertIsNotNone(helper._busy_stop_source, "no deferred stop armed")

    def test_a_fast_rewrite_renders_several_frames(self):
        helper = self._helper()
        helper.set_busy(True)
        GLib.timeout_add(80, lambda: (helper.set_busy(False), False)[1])
        _run_main_loop(config.TRAY_BUSY_MIN_MS + config.TRAY_BUSY_FRAME_MS * 2)
        self.assertGreater(len(self._painted(helper)), 1,
                           "a single frame is invisible to the user")

    # -- lifecycle ---------------------------------------------------------

    def test_the_hold_ends_and_restores_the_status_icon(self):
        helper = self._helper()
        helper.set_busy(True)
        helper.set_busy(False)
        helper._end_busy()                      # what the deferred timer runs
        self.assertIsNone(helper._busy_source)
        self.assertIsNone(helper._busy_stop_source)
        self.assertNotIn("busy", self._painted(helper)[-1],
                         "icon stayed on a pulse frame")

    def test_a_slow_rewrite_runs_its_full_length(self):
        # The minimum is a floor, never a ceiling.
        helper = self._helper()
        helper.set_busy(True)
        _run_main_loop(config.TRAY_BUSY_MIN_MS + config.TRAY_BUSY_FRAME_MS * 3)
        self.assertIsNotNone(helper._busy_source, "the pulse stopped on its own")
        helper.set_busy(False)
        helper._end_busy()
        self.assertNotIn("busy", self._painted(helper)[-1])

    def test_a_second_rewrite_cancels_the_pending_stop(self):
        # Back-to-back rewrites must not let a stale stop kill the new pulse.
        helper = self._helper()
        helper.set_busy(True)
        helper.set_busy(False)
        self.assertIsNotNone(helper._busy_stop_source)
        helper.set_busy(True)
        self.assertIsNone(helper._busy_stop_source, "the stale stop survived")
        self.assertIsNotNone(helper._busy_source)

    def test_starting_twice_does_not_stack_timers(self):
        helper = self._helper()
        helper.set_busy(True)
        first = helper._busy_source
        helper.set_busy(True)
        self.assertEqual(helper._busy_source, first, "a second timer was armed")

    def test_stopping_when_idle_is_a_no_op(self):
        helper = self._helper()
        helper.set_busy(False)
        self.assertIsNone(helper._busy_source)
        self.assertIsNone(helper._busy_stop_source)

    # -- it must not fight the status icon ---------------------------------

    def test_a_status_change_mid_pulse_does_not_steal_the_icon(self):
        # A health poll landing mid-rewrite would otherwise stamp the status
        # icon over a frame and stall the animation visually.
        helper = self._helper()
        helper.set_busy(True)
        painted_before = len(self._painted(helper))
        helper.set_status(HealthStatus.OFFLINE)
        self.assertEqual(len(self._painted(helper)), painted_before,
                         "the status repaint interrupted the pulse")

    def test_the_status_change_still_applies_once_the_pulse_ends(self):
        helper = self._helper()
        helper.set_busy(True)
        helper.set_status(HealthStatus.OFFLINE)
        helper.set_busy(False)
        helper._end_busy()
        self.assertIn(HealthStatus.OFFLINE.value, self._painted(helper)[-1],
                      "the status queued during the pulse was lost")

    # -- the wire ----------------------------------------------------------

    def test_the_wire_bytes_drive_it(self):
        from draftright.models.tray import TrayCommand
        helper = self._helper()
        helper.handle_line(TrayCommand.BUSY.encode("1"))
        self.assertIsNotNone(helper._busy_source)
        helper.handle_line(TrayCommand.BUSY.encode("0"))
        helper._end_busy()
        self.assertIsNone(helper._busy_source)


if __name__ == "__main__":
    unittest.main()
