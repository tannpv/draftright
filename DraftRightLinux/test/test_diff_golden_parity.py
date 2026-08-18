"""Linux WordDiff must match the shared golden vectors (#107, RULE #1).

The single source of truth is ``shared/diff_golden_vectors.json`` at the repo
root; the macOS (Swift) and Windows (C#) ports assert against the same file, so
the three implementations of the LCS word-diff cannot drift apart. GTK-free.

    python3 test/test_diff_golden_parity.py
"""

import json
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from draftright.models.diff import word_diff

# test/ -> DraftRightLinux/ -> repo root
_GOLDEN = Path(__file__).resolve().parents[2] / "shared" / "diff_golden_vectors.json"


class DiffGoldenParityTest(unittest.TestCase):
    def test_every_case_matches_the_shared_vectors(self):
        doc = json.loads(_GOLDEN.read_text(encoding="utf-8"))
        cases = doc["cases"]
        self.assertGreater(len(cases), 0, "golden file is empty")
        for case in cases:
            old_tokens, new_tokens = word_diff(case["old"], case["new"])
            got_old = [[t.text, t.kind.wire_value] for t in old_tokens]
            got_new = [[t.text, t.kind.wire_value] for t in new_tokens]
            self.assertEqual(got_old, case["old_tokens"], f"{case['name']} (old side)")
            self.assertEqual(got_new, case["new_tokens"], f"{case['name']} (new side)")


if __name__ == "__main__":
    unittest.main()
