"""Top-level interaction mode for the rewrite feature (#96).

Mirrors ``AppMode`` on macOS (``DraftRight/AppModel.swift``) and Windows
(``Models/AppMode.cs``) 1:1 — same wire values, same display names — so the
persisted setting means the same thing on every platform.
"""

from __future__ import annotations

from enum import Enum
from typing import Optional


class AppMode(Enum):
    """How the global hotkey behaves.

    ``value`` is the wire/JSON form.  Note ``oneClick`` is camelCase: it is
    what macOS and Windows already persist, and diverging would silently reset
    the mode for anyone syncing settings between platforms.
    """

    ADVANCED = "advanced"
    ONE_CLICK = "oneClick"

    @property
    def display_name(self) -> str:
        """User-facing label.  One-Click is presented as "Simple"."""
        return {
            AppMode.ADVANCED: "Advanced",
            AppMode.ONE_CLICK: "Simple",
        }[self]

    @property
    def description(self) -> str:
        """One-line explanation for the settings row."""
        return {
            AppMode.ADVANCED: "Hotkey opens the panel so you can pick a tone",
            AppMode.ONE_CLICK: "Hotkey rewrites instantly with your preset tone",
        }[self]

    @classmethod
    def from_wire(cls, value: Optional[str]) -> "AppMode":
        """Parse a persisted value; anything unknown falls back to Advanced.

        Advanced is the safe default: it always shows the panel, so a bad
        value cannot silently rewrite and replace the user's text.
        """
        if not value:
            return cls.ADVANCED
        for mode in cls:
            if mode.value == value:
                return mode
        return cls.ADVANCED
