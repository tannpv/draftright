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

import json
from dataclasses import dataclass, field
from enum import Enum

from draftright import config


class GrammarIssueType(Enum):
    """Category of a finding, with the accent colour used to render it.

    Colours match macOS ``GrammarCheckView``: spelling red, grammar orange,
    style blue.
    """

    SPELLING = ("spelling", "Spelling", config.COLOR_GRAMMAR_SPELLING)
    GRAMMAR = ("grammar", "Grammar", config.COLOR_GRAMMAR_GRAMMAR)
    STYLE = ("style", "Style", config.COLOR_GRAMMAR_STYLE)
    OTHER = ("other", "Other", config.COLOR_GRAMMAR_OTHER)

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


class ScoreBand(Enum):
    """Quality band for the 0-100 score, mirroring the macOS thresholds."""

    GOOD = ("Good", config.COLOR_SUCCESS)
    FAIR = ("Fair", config.COLOR_GRAMMAR_GRAMMAR)
    POOR = ("Needs work", config.COLOR_ERROR)

    def __init__(self, display_name: str, tint_color: str):
        self.display_name = display_name
        self.tint_color = tint_color

    @classmethod
    def for_score(cls, score: int) -> "ScoreBand":
        if score >= config.GRAMMAR_SCORE_GOOD:
            return cls.GOOD
        if score >= config.GRAMMAR_SCORE_FAIR:
            return cls.FAIR
        return cls.POOR


@dataclass
class GrammarResult:
    """The ``grammar`` object returned by /rewrite for tone=grammar_check."""

    score: int = 0
    issues: list[GrammarIssue] = field(default_factory=list)
    # The backend emits {"score": 0, "issues": [], "error": ...} when the model
    # returned unparseable JSON. Without carrying it, that failure renders as
    # an impressively confident score of zero with nothing flagged.
    error: str | None = None

    @classmethod
    def from_wire(cls, payload: dict | None) -> "GrammarResult | None":
        if not payload:
            return None
        raw_error = payload.get("error")
        return cls(
            score=_clamp_score(int(payload.get("score") or 0)),
            issues=[GrammarIssue.from_wire(i) for i in payload.get("issues") or []],
            error=str(raw_error) if raw_error else None,
        )

    @classmethod
    def from_json_text(cls, text: str) -> "GrammarResult":
        """Parse the JSON string the API layer carries grammar results in.

        Every tone's result travels as ``str`` so the client rewrite cache
        stays a plain str→str map (as on macOS); for grammar_check that string
        is the analysis itself.
        """
        try:
            payload = json.loads(text)
        except json.JSONDecodeError as exc:
            raise ValueError(f"grammar result is not valid JSON: {exc}") from exc
        result = cls.from_wire(payload)
        if result is None:
            raise ValueError("grammar result is empty")
        return result

    @property
    def band(self) -> ScoreBand:
        return ScoreBand.for_score(self.score)

    @property
    def score_display(self) -> str:
        return f"{self.score}/{config.GRAMMAR_SCORE_MAX}"

    @property
    def issue_count_display(self) -> str:
        count = len(self.issues)
        return f"{count} issue" if count == 1 else f"{count} issues"


def _clamp_score(score: int) -> int:
    return max(0, min(config.GRAMMAR_SCORE_MAX, score))
