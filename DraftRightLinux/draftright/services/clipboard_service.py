"""
Clipboard operations for X11 and Wayland.

Uses command-line tools (``xsel`` / ``xclip`` / ``wl-copy`` / ``wl-paste``)
via subprocess so there are no native library dependencies beyond what a
typical Linux desktop already ships.
"""

from __future__ import annotations

import logging
import os
import shutil
import subprocess
import time

log = logging.getLogger(__name__)

_TIMEOUT = 3  # seconds for clipboard tool calls


# ---------------------------------------------------------------------------
# Display-server detection
# ---------------------------------------------------------------------------

def _is_wayland() -> bool:
    if os.environ.get("GDK_BACKEND", "").lower() == "wayland":
        return True
    if os.environ.get("WAYLAND_DISPLAY"):
        return True
    return False


# ---------------------------------------------------------------------------
# Tool resolution
# ---------------------------------------------------------------------------

def _has(cmd: str) -> bool:
    return shutil.which(cmd) is not None


# ---------------------------------------------------------------------------
# ClipboardService
# ---------------------------------------------------------------------------

class ClipboardService:
    """Read and write the X11/Wayland clipboard and primary selection."""

    def __init__(self) -> None:
        self._wayland = _is_wayland()

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
        if _has("xsel"):
            return self._run_read(["xsel", "--clipboard", "--output"])
        if _has("xclip"):
            return self._run_read(["xclip", "-selection", "clipboard", "-o"])
        log.warning("No clipboard tool found (need xsel, xclip, or wl-paste).")
        return ""

    def set_clipboard(self, text: str) -> None:
        """Write *text* to the CLIPBOARD selection."""
        if self._wayland:
            self._run_write(["wl-copy"], text)
        elif _has("xsel"):
            self._run_write(["xsel", "--clipboard", "--input"], text)
        elif _has("xclip"):
            self._run_write(["xclip", "-selection", "clipboard"], text)
        else:
            log.warning("No clipboard tool found (need xsel, xclip, or wl-copy).")

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _read_primary(self) -> str:
        """Read the PRIMARY selection (highlighted text)."""
        if self._wayland:
            return self._run_read(["wl-paste", "-p", "--no-newline"])
        if _has("xsel"):
            return self._run_read(["xsel", "--primary", "--output"])
        if _has("xclip"):
            return self._run_read(["xclip", "-selection", "primary", "-o"])
        return ""

    def _simulate_copy(self) -> None:
        """Simulate Ctrl+C to copy the current selection to CLIPBOARD."""
        if self._wayland:
            if _has("wtype"):
                subprocess.run(
                    ["wtype", "-M", "ctrl", "-P", "c", "-m", "ctrl"],
                    timeout=_TIMEOUT,
                    check=False,
                )
                return
        # X11 or Wayland fallback via xdotool
        if _has("xdotool"):
            subprocess.run(
                ["xdotool", "key", "--clearmodifiers", "ctrl+c"],
                timeout=_TIMEOUT,
                check=False,
            )

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
