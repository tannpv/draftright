"""Backend health/connection state.

Single source for the status the health-check probe produces and the tray
displays — replaces the raw ``"connected"`` / ``"offline"`` / ``"wrong_server"``
strings that were threaded through api_client → application → tray_icon.
Rule #1: enums, not literals.
"""

from __future__ import annotations

from enum import Enum


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

    @classmethod
    def from_wire(cls, value: str) -> "HealthStatus":
        """Parse a wire value; unknown → OFFLINE (safe default)."""
        for s in cls:
            if s.value == value:
                return s
        return cls.OFFLINE
