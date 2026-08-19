"""Linux grammar fixAll must match the shared golden vectors (#107, RULE #1).

`parity/grammar-fixall-vectors.json` at the repo root is the single
source of truth; the macOS (Swift) and Windows (C#) ports assert against the
same file, so the three copies of the apply-all logic cannot drift. GTK-free.

    python3 test/test_grammar_fixall_golden_parity.py
"""

import json
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from draftright.models.grammar import GrammarIssue
from draftright.services.grammar_fixer import fix_all

_GOLDEN = Path(__file__).resolve().parents[2] / "parity" / "grammar-fixall-vectors.json"


class GrammarFixAllGoldenParityTest(unittest.TestCase):
    def test_every_case_matches_the_shared_vectors(self):
        doc = json.loads(_GOLDEN.read_text(encoding="utf-8"))
        cases = doc["cases"]
        self.assertGreater(len(cases), 0, "golden file is empty")
        for case in cases:
            issues = [
                GrammarIssue(original=i["original"], suggestion=i["suggestion"],
                             offset=i["offset"])
                for i in case["issues"]
            ]
            got = fix_all(case["text"], issues)
            self.assertEqual(got, case["expected"], case["name"])


if __name__ == "__main__":
    unittest.main()
