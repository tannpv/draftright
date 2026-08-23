#!/usr/bin/env python3
"""Regenerate the WordDiff golden-vector parity file (#107, RULE #1).

`parity/word-diff-vectors.json` is the single source of truth for the expected
word-diff output. The macOS (Swift), Windows (C#) and Linux (Python) WordDiff
ports each assert their output equals these vectors, so the three copies of one
LCS algorithm cannot silently drift — the #22 failure mode (a duplicated routine
diverging between platforms).

Vectors are generated from the Linux reference impl. If a deliberate behaviour
change is made, update the case list here and rerun; the other platforms' parity
tests will then flag any port that disagrees.

    python3 tools/gen_diff_golden_vectors.py
"""
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "DraftRightLinux"))

from draftright.models.diff import word_diff, DiffKind  # noqa: E402

# (name, old, new) — cover identity, replace, insert, delete, prefix/suffix,
# wholesale change, empties, whitespace padding, unicode, punctuation-as-words.
CASES = [
    ("identical", "the quick brown fox", "the quick brown fox"),
    ("single word replaced", "the quick brown fox", "the slow brown fox"),
    ("word inserted", "the brown fox", "the quick brown fox"),
    ("word deleted", "the quick brown fox", "the brown fox"),
    ("prefix changed", "hello world", "goodbye world"),
    ("suffix changed", "hello world", "hello there"),
    ("all different", "one two three", "four five six"),
    ("empty to text", "", "brand new text"),
    ("text to empty", "old text here", ""),
    ("both empty", "", ""),
    ("leading and trailing spaces", "  padded text  ", "  padded prose  "),
    ("unicode vietnamese", "toi yeu tieng viet", "tôi yêu tiếng việt"),
    ("punctuation kept as words", "hi, world!", "hi, there!"),
]


def _pairs(tokens) -> list[list[str]]:
    return [[t.text, t.kind.wire_value] for t in tokens]


def main() -> None:
    cases = []
    for name, old, new in CASES:
        old_tokens, new_tokens = word_diff(old, new)
        cases.append({
            "name": name, "old": old, "new": new,
            "old_tokens": _pairs(old_tokens),
            "new_tokens": _pairs(new_tokens),
        })
    doc = {
        "_comment": (
            "RULE #1 golden-vector parity guard for WordDiff across macOS / "
            "Windows / Linux (#107). Each platform's WordDiff test asserts its "
            "output equals these vectors, so the three ports cannot drift (the "
            "#22 failure mode). Regenerate with tools/gen_diff_golden_vectors.py, "
            "never hand-edit; kinds are the wire values equal/deleted/inserted."
        ),
        "cases": cases,
    }
    out = ROOT / "parity" / "word-diff-vectors.json"
    out.parent.mkdir(exist_ok=True)
    out.write_text(json.dumps(doc, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(f"wrote {out} ({len(cases)} cases)")


if __name__ == "__main__":
    main()
