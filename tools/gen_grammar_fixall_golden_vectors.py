#!/usr/bin/env python3
"""Regenerate the grammar fixAll golden-vector parity file (#107, RULE #1).

`parity/grammar-fixall-vectors.json` is the single source of truth for
applying a whole set of grammar suggestions to text: each issue is re-resolved
from content (offset only disambiguates duplicates) and replaced, in order,
against the evolving text. Ported three ways — GrammarFix.fixAll (Swift, newly
extracted from the view), GrammarFixer.FixAll (C#), grammar_fixer.fix_all
(Python) — so a shared guard keeps them from drifting.

Unlike the resolveRange vectors these compare the resulting STRING, which is
index-unit-independent, so unicode cases are included.

    python3 tools/gen_grammar_fixall_golden_vectors.py
"""
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "DraftRightLinux"))

from draftright.models.grammar import GrammarIssue  # noqa: E402
from draftright.services.grammar_fixer import fix_all  # noqa: E402

# (name, text, [(original, suggestion, offset), ...])
CASES = [
    ("single fix", "teh cat", [("teh", "the", 0)]),
    ("two fixes", "teh cat runned", [("teh", "the", 0), ("runned", "ran", 8)]),
    ("no issues leaves text unchanged", "already fine", []),
    ("stale issue is a no-op", "the cat", [("dog", "hound", 0)]),
    ("duplicate original, offset picks second",
     "is it is it", [("is", "was", 6)]),
    ("duplicate original, offset picks first",
     "is it is it", [("is", "was", 0)]),
    ("sequential fixes on evolving text",
     "a b c", [("a", "x", 0), ("b", "y", 2), ("c", "z", 4)]),
    ("unicode suggestion", "toi yeu", [("toi", "tôi", 0), ("yeu", "yêu", 4)]),
    ("suggestion shorter than original", "the quick fox", [("quick", "sly", 4)]),
    ("empty original issue is skipped", "hello", [("", "X", 0)]),
]


def main() -> None:
    cases = []
    for name, text, raw_issues in CASES:
        issues = [GrammarIssue(original=o, suggestion=s, offset=off)
                  for (o, s, off) in raw_issues]
        cases.append({
            "name": name,
            "text": text,
            "issues": [{"original": o, "suggestion": s, "offset": off}
                       for (o, s, off) in raw_issues],
            "expected": fix_all(text, issues),
        })
    doc = {
        "_comment": (
            "RULE #1 golden-vector parity guard for grammar fixAll across macOS "
            "/ Windows / Linux (#107). Each platform asserts its fixAll output "
            "equals these vectors, so the three ports cannot drift. Compares the "
            "resulting string (index-unit independent), so unicode is included. "
            "Regenerate with tools/gen_grammar_fixall_golden_vectors.py."
        ),
        "cases": cases,
    }
    out = ROOT / "parity" / "grammar-fixall-vectors.json"
    out.parent.mkdir(exist_ok=True)
    out.write_text(json.dumps(doc, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(f"wrote {out} ({len(cases)} cases)")


if __name__ == "__main__":
    main()
