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

from draftright import config
from draftright.helpers.display_server import is_wayland
from draftright.helpers.system_input import TextInputSimulator, has_command

log = logging.getLogger(__name__)

_TIMEOUT = config.SUBPROCESS_TIMEOUT


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

    def get_selected_text(self) -> str:
        """Return the currently selected (highlighted) text.

        Strategy:
          1. Read the PRIMARY selection directly.
          2. If empty, fall back to simulating Ctrl+C and reading CLIPBOARD.
        """
        text = self._read_primary()
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
        if self._wayland:
            return self._run_read(["wl-paste", "--no-newline"])
        if has_command("xsel"):
            return self._run_read(["xsel", "--clipboard", "--output"])
        if has_command("xclip"):
            return self._run_read(["xclip", "-selection", "clipboard", "-o"])
        log.warning("No clipboard tool found (need xsel, xclip, or wl-paste).")
        return ""

    def set_clipboard(self, text: str) -> None:
        """Write *text* to the CLIPBOARD selection."""
        if self._wayland:
            self._run_write(["wl-copy"], text)
        elif has_command("xsel"):
            self._run_write(["xsel", "--clipboard", "--input"], text)
        elif has_command("xclip"):
            self._run_write(["xclip", "-selection", "clipboard"], text)
        else:
            log.warning("No clipboard tool found (need xsel, xclip, or wl-copy).")

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

    def _read_primary(self) -> str:
        """Read the PRIMARY selection (highlighted text)."""
        if self._wayland:
            return self._run_read(["wl-paste", "-p", "--no-newline"])
        if has_command("xsel"):
            return self._run_read(["xsel", "--primary", "--output"])
        if has_command("xclip"):
            return self._run_read(["xclip", "-selection", "primary", "-o"])
        return ""

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
