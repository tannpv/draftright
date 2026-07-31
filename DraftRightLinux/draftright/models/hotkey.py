"""Parsed representation of a global hotkey.

The same user-facing keystring (``"Ctrl+Shift+R"``) has to be rendered two
different ways: X11 wants modifier mask names (``control``, ``mod1``), the
xdg-desktop-portal GlobalShortcuts API wants its own spelling (``CTRL``,
``ALT``).  Parsing lived inside the X11 listener and returned X11 names, so
the portal path had nothing to reuse.

Rule #1: parse once here, render per backend.  Adding a backend means adding
a property to :class:`Modifier`, not another parser.
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import Enum
from typing import Optional


class Modifier(Enum):
    """A modifier key, with the spelling each backend expects.

    ``value`` is the canonical lowercase name used in a keystring.
    """

    CTRL = "ctrl"
    SHIFT = "shift"
    ALT = "alt"
    SUPER = "super"

    @property
    def x11_name(self) -> str:
        """Xlib modifier-mask attribute name."""
        return {
            Modifier.CTRL: "control",
            Modifier.SHIFT: "shift",
            Modifier.ALT: "mod1",
            Modifier.SUPER: "mod4",
        }[self]

    @property
    def portal_name(self) -> str:
        """Spelling used in an xdg-desktop-portal shortcut trigger."""
        return {
            Modifier.CTRL: "CTRL",
            Modifier.SHIFT: "SHIFT",
            Modifier.ALT: "ALT",
            Modifier.SUPER: "LOGO",
        }[self]

    @property
    def display_name(self) -> str:
        """User-facing label."""
        return {
            Modifier.CTRL: "Ctrl",
            Modifier.SHIFT: "Shift",
            Modifier.ALT: "Alt",
            Modifier.SUPER: "Super",
        }[self]

    @classmethod
    def from_wire(cls, value: str) -> Optional["Modifier"]:
        """Parse one keystring token; None when it is not a modifier.

        Accepts the aliases that appear in stored settings and in portal
        triggers (``control``, ``mod1``, ``mod4``, ``logo``, ``meta``).
        """
        token = value.strip().lower()
        aliases = {
            "control": cls.CTRL,
            "mod1": cls.ALT,
            "mod4": cls.SUPER,
            "logo": cls.SUPER,
            "meta": cls.SUPER,
            "win": cls.SUPER,
            "cmd": cls.SUPER,
        }
        for modifier in cls:
            if modifier.value == token:
                return modifier
        return aliases.get(token)


@dataclass(frozen=True)
class Hotkey:
    """A modifier combination plus a single key."""

    modifiers: tuple[Modifier, ...]
    key: str

    @classmethod
    def parse(cls, keystring: str) -> "Hotkey":
        """Parse ``"Ctrl+Shift+R"``.

        Unrecognised modifier tokens are dropped rather than raising: a
        malformed stored setting should degrade to a usable hotkey, not stop
        the app from starting.  The final token is always the key.
        """
        parts = [p.strip() for p in keystring.split("+") if p.strip()]
        if not parts:
            raise ValueError(f"empty hotkey: {keystring!r}")
        key = parts[-1]
        modifiers = []
        for token in parts[:-1]:
            modifier = Modifier.from_wire(token)
            if modifier is not None and modifier not in modifiers:
                modifiers.append(modifier)
        return cls(tuple(modifiers), key)

    @property
    def x11_modifiers(self) -> list[str]:
        """Xlib modifier attribute names, in canonical order."""
        return [m.x11_name for m in self.modifiers]

    def to_portal_trigger(self) -> str:
        """Render as an xdg-desktop-portal ``preferred_trigger``.

        The portal spells modifiers in upper case and expects an XKB key name;
        single letters are lower case there (``CTRL+SHIFT+r``).
        """
        key = self.key.lower() if len(self.key) == 1 else self.key
        return "+".join([m.portal_name for m in self.modifiers] + [key])

    @property
    def display_name(self) -> str:
        """User-facing label, e.g. ``Ctrl+Shift+R``."""
        key = self.key.upper() if len(self.key) == 1 else self.key
        return "+".join([m.display_name for m in self.modifiers] + [key])

    @classmethod
    def from_portal_trigger(cls, trigger: str) -> "Hotkey":
        """Parse a trigger the portal reports back (may differ from ours)."""
        return cls.parse(trigger)
