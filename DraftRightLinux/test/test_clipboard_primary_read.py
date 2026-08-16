"""The pencil's selection read must be side-effect free (#188). GTK/X11-free.

    python3 test/test_clipboard_primary_read.py

``PencilTrigger`` polls its ``read_selection`` several times a second, so that
callable may only *observe* the selection. ``get_selected_text`` may not be used
there: when PRIMARY is empty it synthesises Ctrl+C and rewrites CLIPBOARD, which
on a timer injects a keystroke into whatever window has focus (SIGINT in a
terminal) and fires rewrites from stale clipboard text.

Two halves, because the wiring lives in ``application.py`` and that module
imports ``gi`` — unavailable here:
  * ``PrimaryReadIsObservationOnlyTest`` pins the service contract;
  * ``PencilWiringTest`` reads the wiring with ``ast``, no import needed.
"""

import ast
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from draftright.services import clipboard_service as clipboard_module
from draftright.services.clipboard_service import ClipboardService, Selection

APP_SOURCE = Path(__file__).resolve().parent.parent / "draftright" / "application.py"

# Derived, not spelled out: renaming either method updates this test with it.
SAFE_READ = ClipboardService.get_primary_selection.__name__
UNSAFE_READ = ClipboardService.get_selected_text.__name__


class _SpyService(ClipboardService):
    """A service that records the keystrokes it would synthesise."""

    def __init__(self) -> None:
        # Bypass ClipboardService.__init__: it probes the live display server.
        self._wayland = False
        self._sim = None
        self._injector = None
        self.side_effects: list[str] = []

    def _simulate_copy(self) -> None:
        self.side_effects.append("ctrl+c")


class PrimaryReadIsObservationOnlyTest(unittest.TestCase):
    """Patches the shared tool runner, so no real clipboard is ever touched."""

    def _service(self, primary: str = "", clipboard: str = "") -> _SpyService:
        svc = _SpyService()

        def fake_read(cmd):
            # Every tool names the selection in its argv; match on the enum
            # value rather than a copy of any one tool's flag spelling.
            return primary if Selection.PRIMARY.value in " ".join(cmd) else clipboard

        def fake_run(cmd, stdin_text=None):
            svc.side_effects.append(f"wrote {' '.join(cmd)}")
            return True

        for name, fake in (("read_tool", fake_read), ("run_tool", fake_run)):
            real = getattr(clipboard_module, name)
            setattr(clipboard_module, name, fake)
            self.addCleanup(setattr, clipboard_module, name, real)
        return svc

    def setUp(self):
        # The dev box may have no clipboard tool installed; pretend the
        # preferred one is there so the real dispatch runs into the stub.
        real_has_command = clipboard_module.has_command
        clipboard_module.has_command = lambda name: True
        self.addCleanup(setattr, clipboard_module, "has_command", real_has_command)

    def test_returns_the_primary_selection(self):
        svc = self._service(primary="highlighted")
        self.assertEqual(svc.get_primary_selection(), "highlighted")
        self.assertEqual(svc.side_effects, [])

    def test_empty_primary_stays_silent(self):
        # The poll-loop case: PRIMARY is empty most of the time on an idle
        # desktop. No synthetic keystroke, no clipboard write, ever.
        svc = self._service(primary="", clipboard="stale clipboard text")
        self.assertEqual(svc.get_primary_selection(), "")
        self.assertEqual(svc.side_effects, [])

    def test_get_selected_text_keeps_its_ctrl_c_fallback(self):
        # Not a bug — the hotkey is a deliberate user action, so paying the
        # side effects to recover a selection X11 did not publish is correct.
        svc = self._service(primary="", clipboard="copied")
        self.assertEqual(svc.get_selected_text(), "copied")
        self.assertIn("ctrl+c", svc.side_effects)


class PencilWiringTest(unittest.TestCase):
    """application.py must hand PencilTrigger the observation-only read."""

    def _pencil_read_selection_arg(self) -> str:
        tree = ast.parse(APP_SOURCE.read_text())
        for node in ast.walk(tree):
            if not isinstance(node, ast.Call):
                continue
            if getattr(node.func, "id", None) != "PencilTrigger":
                continue
            for kw in node.keywords:
                if kw.arg == "read_selection":
                    return ast.unparse(kw.value)
        self.fail("No PencilTrigger(read_selection=...) call found in application.py")

    def test_pencil_polls_primary_not_the_ctrl_c_capture(self):
        wired = self._pencil_read_selection_arg()
        self.assertTrue(
            wired.endswith(f".{SAFE_READ}"),
            f"PencilTrigger must poll {SAFE_READ}, got {wired!r}",
        )
        self.assertNotIn(UNSAFE_READ, wired)


if __name__ == "__main__":
    unittest.main()
