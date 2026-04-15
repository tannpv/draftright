"""
Replace selected text in the source application by pasting from clipboard.

Flow:
  1. Copy *text* to the system clipboard.
  2. Brief delay (100 ms) for clipboard to settle.
  3. Simulate Ctrl+V via ``xdotool`` (X11) or ``wtype`` (Wayland).
"""

from __future__ import annotations

import logging
import os
import shutil
import subprocess
import time

log = logging.getLogger(__name__)

_TIMEOUT = 3  # seconds


def _is_wayland() -> bool:
    if os.environ.get("GDK_BACKEND", "").lower() == "wayland":
        return True
    if os.environ.get("WAYLAND_DISPLAY"):
        return True
    return False


def _has(cmd: str) -> bool:
    return shutil.which(cmd) is not None


class TextInjector:
    """Paste replacement text into the active application."""

    def __init__(self) -> None:
        self._wayland = _is_wayland()

    def inject_text(self, text: str) -> None:
        """Set the clipboard to *text* and simulate a paste keystroke."""
        self._set_clipboard(text)
        time.sleep(0.1)  # 100 ms for the clipboard to settle
        self._simulate_paste(text)

    # ------------------------------------------------------------------
    # Clipboard write
    # ------------------------------------------------------------------

    def _set_clipboard(self, text: str) -> None:
        try:
            if self._wayland:
                subprocess.run(
                    ["wl-copy"], input=text, text=True,
                    timeout=_TIMEOUT, check=False,
                )
            elif _has("xsel"):
                subprocess.run(
                    ["xsel", "--clipboard", "--input"], input=text, text=True,
                    timeout=_TIMEOUT, check=False,
                )
            elif _has("xclip"):
                subprocess.run(
                    ["xclip", "-selection", "clipboard"], input=text, text=True,
                    timeout=_TIMEOUT, check=False,
                )
            else:
                log.warning("No clipboard tool available for text injection.")
        except Exception as exc:
            log.error("Failed to set clipboard: %s", exc)

    # ------------------------------------------------------------------
    # Paste simulation
    # ------------------------------------------------------------------

    def _simulate_paste(self, text: str) -> None:
        if self._wayland:
            if _has("wtype"):
                try:
                    subprocess.run(
                        ["wtype", "-M", "ctrl", "-P", "v", "-m", "ctrl"],
                        timeout=_TIMEOUT, check=False,
                    )
                    return
                except Exception as exc:
                    log.debug("wtype paste failed: %s", exc)

        # X11 or Wayland fallback
        if _has("xdotool"):
            try:
                subprocess.run(
                    ["xdotool", "key", "--clearmodifiers", "ctrl+v"],
                    timeout=_TIMEOUT, check=False,
                )
                return
            except Exception as exc:
                log.debug("xdotool paste failed: %s", exc)

        # Last resort: xdotool type (only practical for short strings)
        if _has("xdotool") and len(text) < 500:
            try:
                subprocess.run(
                    ["xdotool", "type", "--clearmodifiers", text],
                    timeout=_TIMEOUT, check=False,
                )
                return
            except Exception as exc:
                log.debug("xdotool type failed: %s", exc)

        log.error(
            "Cannot inject text — install xdotool (X11) or wtype (Wayland)."
        )
