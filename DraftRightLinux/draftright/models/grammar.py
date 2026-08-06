"""Grammar-check result model — parity with macOS and Windows (#107).

Pure logic: no GTK, no network.

IMPORTANT: do NOT trust the LLM-returned ``offset``/``length``. Models count
tokens or bytes and drift by several characters; trusting them spliced
suggestions into the middle of words ("showswincorrectlyectly", BR#49).
Ranges are re-resolved from ``original`` CONTENT at apply time — see
``draftright/services/grammar_fixer.py``. The offset survives only as a
tie-breaker between duplicate occurrences.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum


class GrammarIssueType(Enum):
    """Category of a finding, with the accent colour used to render it.

    Colours match macOS ``GrammarCheckView``: spelling red, grammar orange,
    style blue.
    """

    SPELLING = ("spelling", "Spelling", "#ef4444")
    GRAMMAR = ("grammar", "Grammar", "#f59e0b")
    STYLE = ("style", "Style", "#5d87ff")
    OTHER = ("other", "Other", "#94a3b8")

    def __init__(self, wire_value: str, display_name: str, tint_color: str):
        self.wire_value = wire_value
        self.display_name = display_name
        self.tint_color = tint_color

    @classmethod
    def from_wire(cls, value: str | None) -> "GrammarIssueType":
        """Map the backend's ``type`` string.

        Unknown values fall back to OTHER rather than raising — a category
        added server-side must not break an older client.
        """
        if value:
            lowered = value.lower()
            for member in cls:
                if member.wire_value == lowered:
                    return member
        return cls.OTHER


@dataclass
class GrammarIssue:
    """One finding. ``original`` is the anchor; ``offset`` is only a hint."""

    original: str
    suggestion: str
    issue_type: GrammarIssueType = GrammarIssueType.OTHER
    offset: int = 0
    length: int = 0
    reason: str = ""

    @classmethod
    def from_wire(cls, payload: dict) -> "GrammarIssue":
        original = payload.get("original") or ""
        return cls(
            original=original,
            suggestion=payload.get("suggestion") or "",
            issue_type=GrammarIssueType.from_wire(payload.get("type")),
            offset=int(payload.get("offset") or 0),
            length=int(payload.get("length") or len(original)),
            reason=payload.get("reason") or "",
        )


@dataclass
class GrammarResult:
    """The ``grammar`` object returned by /rewrite for tone=grammar_check."""

    score: int = 0
    issues: list[GrammarIssue] = field(default_factory=list)

    @classmethod
    def from_wire(cls, payload: dict | None) -> "GrammarResult | None":
        if not payload:
            return None
        return cls(
            score=int(payload.get("score") or 0),
            issues=[GrammarIssue.from_wire(i) for i in payload.get("issues") or []],
        )
