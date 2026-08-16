"""
Clipboard operations for X11 and Wayland.

Uses command-line tools (``xsel`` / ``xclip`` / ``wl-copy`` / ``wl-paste``)
via subprocess so there are no native library dependencies beyond what a
typical Linux desktop already ships.
"""

from __future__ import annotations

import logging
import subprocess
import time
from enum import Enum
from typing import Callable, NamedTuple

from draftright import config
from draftright.helpers.display_server import is_wayland
from draftright.helpers.system_input import TextInputSimulator, has_command

log = logging.getLogger(__name__)

_TIMEOUT = config.SUBPROCESS_TIMEOUT


class Selection(Enum):
    """An X11/Wayland selection buffer.

    The two are distinct: PRIMARY holds whatever is highlighted right now and
    updates with no user action, CLIPBOARD holds what was explicitly copied.
    Every tool invocation names one of them, so the name lives here once — the
    value doubles as the ``xsel``/``xclip`` selection argument.
    """

    PRIMARY = "primary"
    CLIPBOARD = "clipboard"

    @property
    def wl_flags(self) -> list[str]:
        """``wl-paste``/``wl-copy`` address CLIPBOARD by default, PRIMARY via -p."""
        return ["-p"] if self is Selection.PRIMARY else []


class _ToolSet(NamedTuple):
    """The command-line tools that can carry one operation, per display server.

    Each entry builds the argv for a given :class:`Selection`, so the tool's
    name, its flags and the selection it addresses are written down once.
    """

    wayland: Callable[[Selection], list[str]]
    x11: tuple[Callable[[Selection], list[str]], ...]  # preference order


# ---------------------------------------------------------------------------
# ClipboardService
# ---------------------------------------------------------------------------

class ClipboardService:
    """Read and write the X11/Wayland clipboard and primary selection."""

    def __init__(self, injector=None) -> None:
        """
        Args:
            injector: Optional :class:`RemoteDesktopInjector`.  Required for
                Replace to work on Wayland — see :meth:`inject_text`.
        """
        self._wayland = is_wayland()
        self._sim = TextInputSimulator()
        self._injector = injector

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def get_primary_selection(self) -> str:
        """Read the PRIMARY selection (highlighted text) — observation only.

        Never touches CLIPBOARD and never synthesises a keystroke, so it is safe
        to call on a timer: the pencil trigger (#188) polls this several times a
        second. Use :meth:`get_selected_text` for the one-shot capture a user
        action asks for, where the Ctrl+C fallback is worth its side effects.
        """
        return self._read_selection(Selection.PRIMARY)

    def get_selected_text(self) -> str:
        """Return the currently selected (highlighted) text.

        Strategy:
          1. Read the PRIMARY selection directly.
          2. If empty, fall back to simulating Ctrl+C and reading CLIPBOARD.

        Step 2 has side effects — it types into whatever window has focus and
        briefly overwrites CLIPBOARD — so this belongs on user-initiated capture
        (the hotkey), never on a poll loop. Pollers want
        :meth:`get_primary_selection`.
        """
        text = self.get_primary_selection()
        if text:
            return text

        # Fallback: save clipboard, simulate Ctrl+C, read, restore.
        saved = self.get_clipboard()
        self._simulate_copy()
        time.sleep(0.15)
        text = self.get_clipboard()
        # Restore original clipboard content.
        if saved:
            self.set_clipboard(saved)
        return text

    def get_clipboard(self) -> str:
        """Read the CLIPBOARD selection."""
        return self._read_selection(Selection.CLIPBOARD)

    def set_clipboard(self, text: str) -> None:
        """Write *text* to the CLIPBOARD selection."""
        self._write_selection(Selection.CLIPBOARD, text)

    def inject_text(self, text: str, on_done=None) -> None:
        """Replace the current selection in the source app with *text*.

        Copies *text* to the clipboard, lets it settle, then sends Ctrl+V.
        This is what the rewrite panel's "Replace" action calls.

        On Wayland the keystroke must go through the RemoteDesktop portal:
        ``xdotool`` reaches only XWayland clients, so a paste aimed at a native
        Wayland window silently does nothing.  That path is asynchronous (it
        may prompt for permission), hence *on_done*.

        Args:
            text: The replacement text.
            on_done: Optional callback receiving True when the keystroke was
                delivered, False when it was not.  The text is on the clipboard
                either way, so the user can always paste manually.
        """
        self.set_clipboard(text)
        time.sleep(0.1)  # 100 ms for the clipboard to settle before paste

        if self._wayland and self._injector is not None:
            self._injector.paste(on_done)
            return

        delivered = self._sim.paste()
        if not delivered:
            # No paste tool → last-resort type (short strings only).
            delivered = self._sim.type_text(text)
        if on_done is not None:
            on_done(delivered)

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _read_selection(self, selection: Selection) -> str:
        """Read *selection* using whichever clipboard tool is installed.

        The one place that knows how each tool names a selection — the public
        readers are thin wrappers over this, so adding a tool or a selection is
        a single edit and the two can never disagree.
        """
        cmd = self._pick_command(selection, self._READ_COMMANDS)
        return self._run_read(cmd) if cmd else ""

    def _write_selection(self, selection: Selection, text: str) -> None:
        """Write *text* into *selection*, mirroring :meth:`_read_selection`."""
        cmd = self._pick_command(selection, self._WRITE_COMMANDS)
        if cmd:
            self._run_write(cmd, text)

    def _pick_command(self, selection: Selection, tools: _ToolSet) -> list[str]:
        """Return the argv of the first usable tool, or [] with a warning.

        The X tools cannot see a native Wayland client's selections, so the two
        sets are never mixed: Wayland has exactly one tool, X11 falls back
        through its list in preference order.
        """
        if self._wayland:
            return tools.wayland(selection)
        for build in tools.x11:
            argv = build(selection)
            # argv[0] is the tool name, so it and its flags stay together.
            if has_command(argv[0]):
                return argv
        log.warning(
            "No clipboard tool found (need %s).",
            " or ".join(build(selection)[0] for build in tools.x11),
        )
        return []

    _READ_COMMANDS = _ToolSet(
        wayland=lambda sel: ["wl-paste", *sel.wl_flags, "--no-newline"],
        x11=(
            lambda sel: ["xsel", f"--{sel.value}", "--output"],
            lambda sel: ["xclip", "-selection", sel.value, "-o"],
        ),
    )
    _WRITE_COMMANDS = _ToolSet(
        wayland=lambda sel: ["wl-copy", *sel.wl_flags],
        x11=(
            lambda sel: ["xsel", f"--{sel.value}", "--input"],
            lambda sel: ["xclip", "-selection", sel.value],
        ),
    )

    def _simulate_copy(self) -> None:
        """Simulate Ctrl+C to copy the current selection to CLIPBOARD."""
        self._sim.copy()

    @staticmethod
    def _run_read(cmd: list[str]) -> str:
        try:
            result = subprocess.run(
                cmd, capture_output=True, text=True, timeout=_TIMEOUT,
            )
            return result.stdout if result.returncode == 0 else ""
        except FileNotFoundError:
            return ""
        except subprocess.TimeoutExpired:
            return ""
        except Exception as exc:
            log.debug("Clipboard read error (%s): %s", cmd[0], exc)
            return ""

    @staticmethod
    def _run_write(cmd: list[str], text: str) -> None:
        try:
            subprocess.run(
                cmd, input=text, text=True, timeout=_TIMEOUT, check=False,
            )
        except FileNotFoundError:
            log.warning("Clipboard tool not found: %s", cmd[0])
        except Exception as exc:
            log.debug("Clipboard write error (%s): %s", cmd[0], exc)
