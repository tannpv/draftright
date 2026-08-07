"""Applying grammar suggestions — port of the macOS resolve/apply logic (#107).

Pure logic: no GTK, no network, so it is unit-testable without a display.
Mirrors ``DraftRightWindows/DraftRightWindows/Services/GrammarFixer.cs``.

LLM-reported offsets are UNRELIABLE — models count tokens or bytes and drift
by several characters. Trusting them spliced suggestions into the middle of
words ("showswincorrectlyectly", BR#49). Every range is therefore re-resolved
from the issue's ``original`` CONTENT against the current text at the moment
of use; the numeric offset only disambiguates duplicate occurrences, where the
nearest one wins.

See memory ``feedback_llm_offsets_unreliable``.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Iterable

from draftright.models.grammar import GrammarIssue


def resolve_range(issue: GrammarIssue, text: str) -> tuple[int, int] | None:
    """Locate ``issue.original`` in ``text`` as ``(start, length)``.

    Prefers the occurrence nearest the LLM-claimed offset. Returns None when
    the original no longer exists — a stale issue, e.g. one already removed by
    an overlapping fix.
    """
    if issue is None or not issue.original or text is None:
        return None

    candidates: list[int] = []
    start = 0
    while True:
        at = text.find(issue.original, start)
        if at < 0:
            break
        candidates.append(at)
        # Advance past this match: overlapping matches would yield ranges that
        # cannot both be applied.
        start = at + len(issue.original)
    if not candidates:
        return None

    claimed = max(0, min(issue.offset, len(text)))
    # min() is stable, so ties keep the earliest occurrence — same as macOS.
    best = min(candidates, key=lambda c: abs(c - claimed))
    return best, len(issue.original)


def apply_fix(text: str, issue: GrammarIssue) -> str:
    """Apply one suggestion. Returns ``text`` unchanged when the issue is stale."""
    found = resolve_range(issue, text)
    if found is None:
        return text
    start, length = found
    return text[:start] + issue.suggestion + text[start + length:]


@dataclass(frozen=True)
class FixOutcome:
    """What ``apply_all`` actually managed to do.

    ``skipped`` is the part a plain "fixed text" return cannot express: when
    two suggestions overlap, applying the first destroys the span the second
    anchors to, so the second can never apply. Reporting it is what stops the
    UI announcing "All issues fixed!" over text that is still wrong — and what
    stops those suggestions vanishing with no explanation.
    """

    text: str
    applied: list[GrammarIssue]
    skipped: list[GrammarIssue]

    @property
    def is_complete(self) -> bool:
        """True when every issue was applied and none had to be skipped."""
        return not self.skipped


def apply_all(text: str, issues: Iterable[GrammarIssue]) -> FixOutcome:
    """Apply every issue, re-resolving against the evolving text each time.

    Order does not matter because ranges come from content rather than
    offsets — which is exactly what the offset-based version got wrong.
    """
    result = text
    applied: list[GrammarIssue] = []
    skipped: list[GrammarIssue] = []
    for issue in issues or []:
        if resolve_range(issue, result) is None:
            skipped.append(issue)
            continue
        result = apply_fix(result, issue)
        applied.append(issue)
    return FixOutcome(text=result, applied=applied, skipped=skipped)


def fix_all(text: str, issues: Iterable[GrammarIssue]) -> str:
    """Apply every issue and return just the corrected text.

    Thin wrapper over :func:`apply_all` so the loop exists once; callers that
    need to know what was skipped use ``apply_all`` directly.
    """
    return apply_all(text, issues).text


def remaining_issues(text: str, issues: Iterable[GrammarIssue]) -> list[GrammarIssue]:
    """Issues still applicable to ``text``.

    Lets the UI drop stale entries after a fix with no offset bookkeeping.
    """
    if not issues:
        return []
    return [i for i in issues if resolve_range(i, text) is not None]
