"""Contract between the GTK4 app and its GTK3 tray-helper process.

The tray runs out-of-process (Ayatana AppIndicator3 is GTK3-only and cannot
be loaded beside GTK4), so the two halves talk over a session-bus action group
and a stdin pipe.  Both halves import *this* module: the action names and the
pipe grammar exist once, so renaming a menu entry cannot silently break the
other process.

Rule #1: enums + a single source, never raw strings threaded across modules.
Adding a menu entry = add a :class:`TrayAction` member and a handler in
``application.py``; the helper picks it up with no further edits.
"""

from __future__ import annotations

from enum import Enum
from typing import Optional


class TrayAction(Enum):
    """A ``Gio.SimpleAction`` the app exports and the tray menu activates.

    ``value`` is the GAction name on the bus; :attr:`display_name` is the menu
    label.  Declaration order is menu order.
    """

    SHOW = "show"
    SETTINGS = "settings"
    SUGGEST_FEATURE = "suggest-feature"
    REPORT_BUG = "report-bug"
    SIGN_OUT = "sign-out"
    QUIT = "quit"

    @property
    def display_name(self) -> str:
        """User-facing menu label."""
        return {
            TrayAction.SHOW: "Show DraftRight",
            TrayAction.SETTINGS: "Open Settings",
            TrayAction.SUGGEST_FEATURE: "Suggest a feature…",
            TrayAction.REPORT_BUG: "Report a bug…",
            TrayAction.SIGN_OUT: "Sign Out",
            TrayAction.QUIT: "Quit",
        }[self]

    @property
    def starts_group(self) -> bool:
        """True when a separator belongs above this entry."""
        return self in (TrayAction.SHOW, TrayAction.SIGN_OUT)

    @classmethod
    def from_wire(cls, value: str) -> Optional["TrayAction"]:
        """Parse an action name; None when unknown."""
        for action in cls:
            if action.value == value:
                return action
        return None


class TrayCommand(Enum):
    """Verb in a parent → helper pipe message (``<verb>:<payload>``)."""

    STATUS = "status"
    # "1"/"0" — an app update is ready to install (#22).
    UPDATE = "update"
    # "1"/"0" — a One-Click rewrite is in flight, so pulse the icon (#6).
    # Transient and outranks the status tint while it lasts; the helper
    # restores the status icon when it clears.
    BUSY = "busy"
    QUIT = "quit"

    def encode(self, payload: str = "") -> str:
        """Render one newline-terminated protocol line."""
        return f"{self.value}:{payload}\n" if payload else f"{self.value}\n"

    @classmethod
    def parse(cls, line: str) -> tuple[Optional["TrayCommand"], str]:
        """Split a received line into (command, payload).

        Returns ``(None, "")`` for anything unrecognised so the helper can log
        and keep running rather than crash on a malformed line.
        """
        verb, _, payload = line.strip().partition(":")
        for command in cls:
            if command.value == verb:
                return command, payload
        return None, ""
