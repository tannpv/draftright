"""Cross-platform parity for the RewriteCache key (issue #174).

The key algorithm is duplicated in three languages — macOS (Swift), Windows
(C#) and Linux (Python). This test asserts the Python ``RewriteCache._key``
reproduces every vector in the shared ``parity/rewrite-cache-key-vectors.json``
fixture, the single source of truth all three platforms check against. If it
fails, the Python key format drifted from the others — fix the code, not the
fixture (unless the format was changed on purpose in all three).

Runnable without a display / GTK:  python3 -m unittest discover test
"""

import json
import sys
import unittest
from pathlib import Path

# Make the package importable when the file is run directly (matches the other
# tests in this dir), not only under `python3 -m unittest` with PYTHONPATH set.
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from draftright.services.rewrite_cache import RewriteCache  # noqa: E402


def _fixture_path() -> Path:
    """Walk up from this file to the repo-root ``parity/`` fixture."""
    here = Path(__file__).resolve()
    for parent in (here, *here.parents):
        candidate = parent / "parity" / "rewrite-cache-key-vectors.json"
        if candidate.exists():
            return candidate
    raise FileNotFoundError(
        f"parity/rewrite-cache-key-vectors.json not found walking up from {here}"
    )


class RewriteCacheKeyParityTest(unittest.TestCase):
    def test_key_matches_shared_golden_vectors(self):
        vectors = json.loads(_fixture_path().read_text(encoding="utf-8"))["vectors"]
        self.assertTrue(vectors, "fixture must contain at least one vector")
        for v in vectors:
            with self.subTest(text=v["text"], tone=v["tone"], language=v["language"]):
                self.assertEqual(
                    v["expectedKey"],
                    RewriteCache._key(v["text"], v["tone"], v["language"]),
                )


if __name__ == "__main__":
    unittest.main()
