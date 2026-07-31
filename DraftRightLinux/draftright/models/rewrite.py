"""Typed view of the ``POST /rewrite`` response.

The endpoint returns ``{rewritten_text, usage_today, daily_limit}``, but the
panel passed that whole dict to ``Gtk.TextBuffer.set_text()``, which needs a
str — so every *successful* rewrite raised ``TypeError: Must be string, not
dict``.  It went unnoticed because a rewrite had never actually run: the app
was never signed in, so the call never reached the success path.

Parsing in one typed place (Rule #1) means callers take ``.text`` and cannot
re-introduce that mistake, and the usage counters stop being silently dropped.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Optional


@dataclass(frozen=True)
class RewriteResult:
    """One rewrite returned by the backend."""

    text: str
    usage_today: Optional[int] = None
    daily_limit: Optional[int] = None

    @classmethod
    def from_wire(cls, payload: dict | str) -> "RewriteResult":
        """Parse a ``/rewrite`` response.

        Accepts a bare string too, so a cached value or a backend that returns
        plain text still works rather than crashing the panel.
        """
        if isinstance(payload, str):
            return cls(text=payload)
        if not isinstance(payload, dict):
            raise ValueError(f"unexpected /rewrite response: {type(payload).__name__}")

        text = payload.get("rewritten_text")
        if not isinstance(text, str):
            raise ValueError("/rewrite response has no rewritten_text")

        return cls(
            text=text,
            usage_today=_as_int(payload.get("usage_today")),
            daily_limit=_as_int(payload.get("daily_limit")),
        )

    @property
    def usage_display(self) -> str:
        """``"3 / 50 today"``, or "" when the backend omitted the counters."""
        if self.usage_today is None or self.daily_limit is None:
            return ""
        return f"{self.usage_today} / {self.daily_limit} today"


def _as_int(value) -> Optional[int]:
    """Coerce a wire number, tolerating strings and nulls."""
    if value is None:
        return None
    try:
        return int(value)
    except (TypeError, ValueError):
        return None
