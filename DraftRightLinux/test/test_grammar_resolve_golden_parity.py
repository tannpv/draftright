"""Linux grammar resolveRange must match the shared golden vectors (#107, RULE #1).

`shared/grammar_resolve_golden_vectors.json` at the repo root is the single
source of truth; the macOS (Swift) and Windows (C#) ports assert against the
same file, so the three copies of the content-first resolve logic (which the
LLM-offset gotcha, BR#49, makes easy to get subtly wrong) cannot drift. GTK-free.

    python3 test/test_grammar_resolve_golden_parity.py
"""

import json
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from draftright.models.grammar import GrammarIssue
from draftright.services.grammar_fixer import resolve_range

_GOLDEN = Path(__file__).resolve().parents[2] / "shared" / "grammar_resolve_golden_vectors.json"


class GrammarResolveGoldenParityTest(unittest.TestCase):
    def test_every_case_matches_the_shared_vectors(self):
        doc = json.loads(_GOLDEN.read_text(encoding="utf-8"))
        cases = doc["cases"]
        self.assertGreater(len(cases), 0, "golden file is empty")
        for case in cases:
            issue = GrammarIssue(original=case["original"], suggestion="X",
                                 offset=case["offset"])
            r = resolve_range(issue, case["text"])
            got = None if r is None else {"start": r[0], "length": r[1]}
            self.assertEqual(got, case["expected"], case["name"])


if __name__ == "__main__":
    unittest.main()
