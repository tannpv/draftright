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

import json
from dataclasses import dataclass
from typing import Optional


@dataclass(frozen=True)
class RewriteResult:
    """One rewrite returned by the backend.

    ``text`` is the payload every caller wants: the rewritten string for the
    text tones, and the grammar analysis as a JSON string for Grammar Check.
    Carrying both as ``str`` mirrors the macOS ``BackendClient``, and keeps the
    client cache a plain ``str -> str`` map for every tone alike.
    """

    text: str
    usage_today: Optional[int] = None
    daily_limit: Optional[int] = None

    @classmethod
    def from_wire(
        cls, payload: dict | str, *, expects_grammar: bool = False
    ) -> "RewriteResult":
        """Parse a ``/rewrite`` response.

        Accepts a bare string too, so a cached value or a backend that returns
        plain text still works rather than crashing the panel.

        Grammar Check answers with ``{"grammar": {...}}`` and no
        ``rewritten_text`` — pass ``expects_grammar=True`` (from
        :attr:`Tone.returns_grammar_analysis`) for it. The shape is checked
        against what was asked for rather than sniffed, so a backend that
        silently stops returning the grammar object surfaces as a contract
        error instead of an empty panel.
        """
        if isinstance(payload, str):
            return cls(text=payload)
        if not isinstance(payload, dict):
            raise ValueError(f"unexpected /rewrite response: {type(payload).__name__}")

        if expects_grammar:
            grammar = payload.get("grammar")
            if isinstance(grammar, dict):
                text = json.dumps(grammar)
            else:
                # A backend *cache hit* answers grammar_check as
                # ``rewritten_text`` holding the analysis JSON verbatim: the
                # Redis entry is written before the grammar branch runs, and
                # the hit path returns it the same way for every tone. Same
                # payload, one encoding layer out — rejecting it made Grammar
                # Check fail for five minutes after each successful run.
                cached = payload.get("rewritten_text")
                if not isinstance(cached, str) or not _is_grammar_analysis(cached):
                    raise ValueError("/rewrite response has no grammar analysis")
                text = cached
        else:
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


def _is_grammar_analysis(text: str) -> bool:
    """True when *text* decodes to something shaped like a grammar analysis.

    Deliberately checks for the analysis's own fields rather than merely "is
    a JSON object". Accepting any object let an unrelated payload through as
    a real result: ``GrammarResult`` defaults a missing score to 0 and issues
    to empty, so the view would cheerfully render "0/100" beside "Your
    writing looks great!" over text nothing had analysed. The contract is
    that a backend which stops returning the analysis produces a visible
    error, not a blank one (#107).
    """
    try:
        body = json.loads(text)
    except (TypeError, ValueError):
        return False
    return isinstance(body, dict) and ("score" in body or "issues" in body)


def _as_int(value) -> Optional[int]:
    """Coerce a wire number, tolerating strings and nulls."""
    if value is None:
        return None
    try:
        return int(value)
    except (TypeError, ValueError):
        return None
