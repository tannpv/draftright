"""Which mechanism triggers a rewrite — the pencil-on-selection or the hotkey.

Mirrors ``TriggerMode`` on macOS (``DraftRight/TriggerMode.swift``) and Windows
(``Models/TriggerMode.cs``) 1:1 — same wire values (``"pencil"`` / ``"hotkey"``),
so the persisted setting means the same thing on every platform (DraftRight #188).

Mutually exclusive: exactly one is active at a time. Decoupled from whether a
hotkey is configured, so switching to the pencil keeps the saved shortcut.

Platform note: the pencil needs global selection detection + a button placed at
the cursor. Wayland forbids both, so **Pencil mode is X11-only** on Linux; under
Wayland the app stays on the hotkey regardless of this setting (see the app
wiring, which gates on ``display_server.is_x11()``).
"""

from __future__ import annotations

from enum import Enum
from typing import Optional


class TriggerMode(Enum):
    PENCIL = "pencil"
    HOTKEY = "hotkey"

    @property
    def uses_pencil(self) -> bool:
        return self is TriggerMode.PENCIL

    @property
    def uses_hotkey(self) -> bool:
        return self is TriggerMode.HOTKEY

    @property
    def display_name(self) -> str:
        return {
            TriggerMode.PENCIL: "Pencil",
            TriggerMode.HOTKEY: "Hotkey",
        }[self]

    @property
    def description(self) -> str:
        return {
            TriggerMode.PENCIL: "Highlight text by dragging to show a rewrite button (X11 only)",
            TriggerMode.HOTKEY: "Select text, then press the shortcut to rewrite",
        }[self]

    @classmethod
    def from_wire(cls, value: Optional[str]) -> "TriggerMode":
        """Parse a persisted value; anything unknown falls back to Hotkey.

        Hotkey is the safe default: it works on both X11 and Wayland, whereas
        the pencil is X11-only. Existing users (hotkey-only until now) are
        unchanged on upgrade.
        """
        if not value:
            return cls.HOTKEY
        for mode in cls:
            if mode.value == value:
                return mode
        return cls.HOTKEY
