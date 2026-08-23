#!/usr/bin/env python3
"""Regenerate the grammar resolveRange golden-vector parity file (#107, RULE #1).

`parity/grammar-resolve-vectors.json` is the single source of truth for
how a grammar issue's `original` is located in the text — content-first, with
the LLM `offset` only breaking ties between duplicate occurrences (nearest wins,
ties keep the earliest). That subtle rule is ported three ways —
GrammarCheckView.resolveRange (Swift), GrammarFixer.ResolveRange (C#),
grammar_fixer.resolve_range (Python) — and getting it wrong splices suggestions
into the wrong place (BR#49). Each platform's test asserts against this file so
the three cannot drift.

Cases are ASCII on purpose: `(start, length)` is then identical whether the port
indexes by grapheme (Swift), UTF-16 unit (C#) or code point (Python), so the
vectors are portable. Unicode index-unit behaviour is each stdlib's own concern,
not this algorithm's.

    python3 tools/gen_grammar_resolve_golden_vectors.py
"""
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "DraftRightLinux"))

from draftright.models.grammar import GrammarIssue  # noqa: E402
from draftright.services.grammar_fixer import resolve_range  # noqa: E402

# (name, text, original, offset)
CASES = [
    ("exact match, offset correct", "the cat sat", "cat", 4),
    ("offset wildly wrong, resolves by content", "the cat sat", "cat", 999),
    ("negative offset clamps to start", "the cat sat", "cat", -50),
    ("duplicate, offset near second", "is it is it", "is", 6),
    ("duplicate, offset near first", "is it is it", "is", 0),
    ("duplicate, equidistant tie keeps earliest", "ab xx ab", "ab", 3),
    ("three occurrences, pick middle", "no no no", "no", 3),
    ("stale original absent -> null", "the cat sat", "dog", 0),
    ("empty original -> null", "the cat sat", "", 0),
    ("whole-string match", "hello", "hello", 0),
    ("overlapping candidates counted non-overlapping", "aaaa", "aa", 3),
]


def main() -> None:
    cases = []
    for name, text, original, offset in CASES:
        issue = GrammarIssue(original=original, suggestion="X", offset=offset)
        r = resolve_range(issue, text)
        cases.append({
            "name": name,
            "text": text,
            "original": original,
            "offset": offset,
            "expected": None if r is None else {"start": r[0], "length": r[1]},
        })
    doc = {
        "_comment": (
            "RULE #1 golden-vector parity guard for grammar resolveRange across "
            "macOS / Windows / Linux (#107). Each platform asserts its resolve "
            "logic matches these vectors so the three ports cannot drift (BR#49 "
            "class bug). Regenerate with tools/gen_grammar_resolve_golden_vectors.py; "
            "ASCII-only so (start,length) is index-unit-portable. null = stale/empty."
        ),
        "cases": cases,
    }
    out = ROOT / "parity" / "grammar-resolve-vectors.json"
    out.parent.mkdir(exist_ok=True)
    out.write_text(json.dumps(doc, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(f"wrote {out} ({len(cases)} cases)")


if __name__ == "__main__":
    main()
