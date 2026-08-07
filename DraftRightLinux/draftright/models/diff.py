"""Word-level LCS diff — port of macOS ``DraftRight/Diff/WordDiff.swift`` (#107).

Pure logic: no GTK, no network, so it is unit-testable without a display.
Mirrors ``DraftRightWindows/DraftRightWindows/Diff/WordDiff.cs`` token for
token, so the before/after view renders identically on both platforms.
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import Enum

from draftright import config


class DiffKind(Enum):
    """How a token changed between the two sides."""

    EQUAL = ("equal", None)
    DELETED = ("deleted", config.COLOR_DIFF_DELETED)
    INSERTED = ("inserted", config.COLOR_DIFF_INSERTED)

    def __init__(self, wire_value: str, tint_color: str | None):
        self.wire_value = wire_value
        # None means "leave it theme-coloured" — same convention as
        # HealthStatus.tint_color in models/health.py.
        self.tint_color = tint_color


@dataclass(frozen=True)
class DiffToken:
    """One run of text plus how it changed."""

    text: str
    kind: DiffKind


def _tokenize(text: str) -> list[str]:
    """Split into words, emitting each whitespace char as its own token.

    Keeping whitespace as tokens means ``"".join(t.text for t in tokens)``
    reproduces the input exactly, so the rendered diff never loses spacing.
    """
    tokens: list[str] = []
    current: list[str] = []
    for char in text:
        if char.isspace():
            if current:
                tokens.append("".join(current))
                current = []
            tokens.append(char)
        else:
            current.append(char)
    if current:
        tokens.append("".join(current))
    return tokens


def _longest_common_subsequence(a: list[str], b: list[str]) -> list[str]:
    m, n = len(a), len(b)
    # Guard the empty case. The Swift original writes `for i in 1...m`, which
    # TRAPS at runtime when m or n is 0 (verified: SIGTRAP) — so diffing
    # against an empty rewrite crashes on macOS. Python would merely produce an
    # empty range, but being explicit documents the divergence.
    if m == 0 or n == 0:
        return []

    dp = [[0] * (n + 1) for _ in range(m + 1)]
    for i in range(1, m + 1):
        for j in range(1, n + 1):
            if a[i - 1] == b[j - 1]:
                dp[i][j] = dp[i - 1][j - 1] + 1
            else:
                dp[i][j] = max(dp[i - 1][j], dp[i][j - 1])

    result: list[str] = []
    i, j = m, n
    while i > 0 and j > 0:
        if a[i - 1] == b[j - 1]:
            result.append(a[i - 1])
            i -= 1
            j -= 1
        elif dp[i - 1][j] > dp[i][j - 1]:
            i -= 1
        else:
            j -= 1
    result.reverse()
    return result


def exceeds_diff_cap(
    old_text: str,
    new_text: str,
    max_tokens: int = config.DIFF_MAX_TOKENS_PER_SIDE,
) -> bool:
    """True when the pair is too large to diff word-by-word.

    Lets a caller say so before rendering, instead of leaving a wholesale
    red/green wall to read as "every word changed".
    """
    return (
        len(_tokenize(old_text)) > max_tokens
        or len(_tokenize(new_text)) > max_tokens
    )


def word_diff(
    old_text: str,
    new_text: str,
    max_tokens: int = config.DIFF_MAX_TOKENS_PER_SIDE,
) -> tuple[list[DiffToken], list[DiffToken]]:
    """Return (old_tokens, new_tokens) marked equal / deleted / inserted.

    The LCS table is O(len(old) × len(new)); a document-sized selection would
    otherwise stall the caller for seconds — on Linux that is the GTK main
    loop. Past *max_tokens* the result degrades to a wholesale replacement
    rather than freezing, which is honest for a rewrite of that size.
    """
    old_words = _tokenize(old_text)
    new_words = _tokenize(new_text)

    if len(old_words) > max_tokens or len(new_words) > max_tokens:
        return (
            [DiffToken(w, DiffKind.DELETED) for w in old_words],
            [DiffToken(w, DiffKind.INSERTED) for w in new_words],
        )

    lcs = _longest_common_subsequence(old_words, new_words)

    old_tokens: list[DiffToken] = []
    new_tokens: list[DiffToken] = []

    oi = ni = li = 0
    while oi < len(old_words) or ni < len(new_words):
        if li < len(lcs):
            while oi < len(old_words) and old_words[oi] != lcs[li]:
                old_tokens.append(DiffToken(old_words[oi], DiffKind.DELETED))
                oi += 1
            while ni < len(new_words) and new_words[ni] != lcs[li]:
                new_tokens.append(DiffToken(new_words[ni], DiffKind.INSERTED))
                ni += 1
            old_tokens.append(DiffToken(lcs[li], DiffKind.EQUAL))
            new_tokens.append(DiffToken(lcs[li], DiffKind.EQUAL))
            oi += 1
            ni += 1
            li += 1
        else:
            while oi < len(old_words):
                old_tokens.append(DiffToken(old_words[oi], DiffKind.DELETED))
                oi += 1
            while ni < len(new_words):
                new_tokens.append(DiffToken(new_words[ni], DiffKind.INSERTED))
                ni += 1

    return old_tokens, new_tokens
