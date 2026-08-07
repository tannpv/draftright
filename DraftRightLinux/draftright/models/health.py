"""Backend health/connection state.

Single source for the status the health-check probe produces and the tray
displays — replaces the raw ``"connected"`` / ``"offline"`` / ``"wrong_server"``
strings that were threaded through api_client → application → tray_icon.
Rule #1: enums, not literals.
"""

from __future__ import annotations

from enum import Enum


class RefreshOutcome(Enum):
    """Why a token refresh ended the way it did.

    A bool could not carry this: "the server rejected the refresh token" and
    "the server could not be reached" both mean *not refreshed*, but only the
    first is a sign-out. Collapsing them made the tray report a sign-out on a
    flaky connection while the account UI still showed a live session.
    """

    REFRESHED = "refreshed"
    REJECTED = "rejected"       # server said no — the session really is over
    UNAVAILABLE = "unavailable"  # could not ask — auth state is simply unknown

    @property
    def may_retry(self) -> bool:
        """True when the caller should re-issue the request it was refreshing."""
        return self is RefreshOutcome.REFRESHED


class HealthStatus(Enum):
    """Result of the /health + /auth/me probe."""

    CONNECTED = "connected"
    NOT_LOGGED_IN = "not_logged_in"
    OFFLINE = "offline"
    WRONG_SERVER = "wrong_server"

    @property
    def display_name(self) -> str:
        """User-facing tray label."""
        return {
            HealthStatus.CONNECTED: "Connected",
            HealthStatus.NOT_LOGGED_IN: "Not Logged In",
            HealthStatus.OFFLINE: "Offline",
            HealthStatus.WRONG_SERVER: "Wrong Server",
        }[self]

    @property
    def tint_color(self) -> "str | None":
        """Hex tint for the tray icon, or None to leave it theme-coloured.

        CONNECTED stays untinted so the normal state is recoloured by the
        shell and sits correctly in light and dark panels; the abnormal
        states are tinted, mirroring the macOS menu-bar symbol.
        """
        return {
            HealthStatus.CONNECTED: None,
            HealthStatus.NOT_LOGGED_IN: "#eab308",   # yellow-500
            HealthStatus.OFFLINE: "#ef4444",         # red-500
            HealthStatus.WRONG_SERVER: "#a855f7",    # purple-500
        }[self]

    @classmethod
    def from_wire(cls, value: str) -> "HealthStatus":
        """Parse a wire value; unknown → OFFLINE (safe default)."""
        for s in cls:
            if s.value == value:
                return s
        return cls.OFFLINE
