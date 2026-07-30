"""
Replace selected text in the source application by pasting from clipboard.

Flow:
  1. Copy *text* to the system clipboard.
  2. Brief delay (100 ms) for clipboard to settle.
  3. Simulate Ctrl+V via ``xdotool`` (X11) or ``wtype`` (Wayland).

Thin wrapper over :class:`ClipboardService` + the shared
:class:`TextInputSimulator`; kept as a standalone entry point for callers that
only need injection. Clipboard writing and keystroke simulation live in one
place each (Rule #1: no duplication).
"""

from __future__ import annotations

import logging

from draftright.services.clipboard_service import ClipboardService

log = logging.getLogger(__name__)


class TextInjector:
    """Paste replacement text into the active application."""

    def __init__(self) -> None:
        self._clipboard = ClipboardService()

    def inject_text(self, text: str) -> None:
        """Set the clipboard to *text* and simulate a paste keystroke."""
        self._clipboard.inject_text(text)
