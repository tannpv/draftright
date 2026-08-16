"""Pencil trigger decision (#188). GTK/X11-free — runs headless.

    python3 test/test_pencil_trigger_decision.py
"""

import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from draftright.services.pencil_trigger_decision import should_trigger


class ShouldTriggerTest(unittest.TestCase):
    def test_new_nonblank_selection_fires(self):
        self.assertTrue(should_trigger(None, "hello"))
        self.assertTrue(should_trigger("old", "new"))
        self.assertTrue(should_trigger("", "hello"))

    def test_unchanged_selection_does_not_refire(self):
        # Same still selection seen on the next poll tick — not a new highlight.
        self.assertFalse(should_trigger("hello", "hello"))

    def test_blank_or_empty_never_fires(self):
        # A plain click clears/empties PRIMARY; nothing to rewrite.
        self.assertFalse(should_trigger("hello", ""))
        self.assertFalse(should_trigger("hello", None))
        self.assertFalse(should_trigger(None, "   "))
        self.assertFalse(should_trigger(None, "\n\t "))

    def test_whitespace_change_around_same_text_still_fires(self):
        # Different non-blank value = a genuinely new selection.
        self.assertTrue(should_trigger("hello", "hello world"))


if __name__ == "__main__":
    unittest.main()
