"""Word diff + grammar fixing (#107).

Deliberately mirrors DraftRightWindows.PureTests/WordDiffTests.cs and
GrammarFixerTests.cs case for case, so a divergence between the two clients
shows up as a test failure rather than as different behaviour on screen.

Imports nothing from GTK, so this module runs on any host with Python.
"""

import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from draftright.models.diff import DiffKind, word_diff  # noqa: E402
from draftright.models.grammar import (  # noqa: E402
    GrammarIssue,
    GrammarIssueType,
    GrammarResult,
)
from draftright.services.grammar_fixer import (  # noqa: E402
    apply_fix,
    fix_all,
    remaining_issues,
    resolve_range,
)


def _rebuild(tokens):
    return "".join(t.text for t in tokens)


class WordDiffTest(unittest.TestCase):
    def test_identical_text_is_all_equal(self):
        old_t, new_t = word_diff("the quick fox", "the quick fox")
        self.assertTrue(all(t.kind is DiffKind.EQUAL for t in old_t))
        self.assertTrue(all(t.kind is DiffKind.EQUAL for t in new_t))

    def test_replaced_word_is_deleted_left_inserted_right(self):
        old_t, new_t = word_diff("the quick fox", "the slow fox")
        self.assertIn(("quick", DiffKind.DELETED), [(t.text, t.kind) for t in old_t])
        self.assertIn(("slow", DiffKind.INSERTED), [(t.text, t.kind) for t in new_t])

    def test_pure_insertion_leaves_left_untouched(self):
        old_t, new_t = word_diff("hello world", "hello brave world")
        self.assertFalse(any(t.kind is DiffKind.DELETED for t in old_t))
        self.assertIn(("brave", DiffKind.INSERTED), [(t.text, t.kind) for t in new_t])

    def test_pure_deletion_leaves_right_untouched(self):
        old_t, new_t = word_diff("hello brave world", "hello world")
        self.assertIn(("brave", DiffKind.DELETED), [(t.text, t.kind) for t in old_t])
        self.assertFalse(any(t.kind is DiffKind.INSERTED for t in new_t))

    def test_tokens_rebuild_the_original_strings_exactly(self):
        a, b = "the  quick\nbrown fox", "the slow\nbrown  fox"
        old_t, new_t = word_diff(a, b)
        self.assertEqual(_rebuild(old_t), a)
        self.assertEqual(_rebuild(new_t), b)

    def test_empty_side_does_not_raise(self):
        # The Swift original traps on `for i in 1...0` (verified: SIGTRAP), so
        # diffing against an empty rewrite crashes on macOS. This port must not.
        for a, b in (("", "hello"), ("hello", ""), ("", "")):
            with self.subTest(a=a, b=b):
                old_t, new_t = word_diff(a, b)
                self.assertEqual(_rebuild(old_t), a)
                self.assertEqual(_rebuild(new_t), b)

    def test_empty_old_side_marks_everything_inserted(self):
        old_t, new_t = word_diff("", "brand new")
        self.assertEqual(old_t, [])
        self.assertTrue(all(t.kind is DiffKind.INSERTED for t in new_t))

    def test_completely_different_text_shares_nothing(self):
        old_t, new_t = word_diff("aaa", "bbb")
        self.assertTrue(all(t.kind is DiffKind.DELETED for t in old_t))
        self.assertTrue(all(t.kind is DiffKind.INSERTED for t in new_t))

    def test_repeated_words_do_not_lose_content(self):
        a, b = "a a a b", "a b b"
        old_t, new_t = word_diff(a, b)
        self.assertEqual(_rebuild(old_t), a)
        self.assertEqual(_rebuild(new_t), b)

    def test_unicode_round_trips(self):
        a, b = "tôi đang viết mã", "tôi đang viết code"
        old_t, new_t = word_diff(a, b)
        self.assertEqual(_rebuild(old_t), a)
        self.assertEqual(_rebuild(new_t), b)

    def test_equal_tokens_are_theme_coloured(self):
        self.assertIsNone(DiffKind.EQUAL.tint_color)
        self.assertEqual(DiffKind.DELETED.tint_color, "#ef4444")


def _issue(original, suggestion, offset=0, issue_type=GrammarIssueType.GRAMMAR):
    return GrammarIssue(
        original=original,
        suggestion=suggestion,
        issue_type=issue_type,
        offset=offset,
        length=len(original),
        reason="test",
    )


class GrammarFixerTest(unittest.TestCase):
    def test_resolves_by_content_when_the_offset_is_wrong(self):
        text = "This sentence shows incorrectly."
        fixed = apply_fix(text, _issue("incorrectly", "correctly", offset=9999))
        self.assertEqual(fixed, "This sentence shows correctly.")
        # The exact splice-into-the-middle-of-a-word failure from BR#49.
        self.assertNotIn("showswincorrectlyectly", fixed)

    def test_offset_disambiguates_duplicates_nearest_wins(self):
        text = "cat dog cat dog cat"          # "cat" at 0, 8, 16
        self.assertEqual(
            apply_fix(text, _issue("cat", "fox", offset=15)), "cat dog cat dog fox"
        )
        self.assertEqual(
            apply_fix(text, _issue("cat", "fox", offset=0)), "fox dog cat dog cat"
        )

    def test_tie_on_distance_prefers_the_earlier_occurrence(self):
        # "ab" at 0 and 4; claimed 2 is equidistant. min() is stable → earliest.
        self.assertEqual(apply_fix("ab..ab", _issue("ab", "XY", offset=2)), "XY..ab")

    def test_stale_issue_leaves_text_unchanged(self):
        self.assertEqual(
            apply_fix("already fixed", _issue("nonexistent", "whatever")),
            "already fixed",
        )

    def test_empty_original_is_never_resolved(self):
        self.assertIsNone(resolve_range(_issue("", "x"), "some text"))

    def test_offset_beyond_text_length_is_clamped(self):
        self.assertEqual(apply_fix("short", _issue("short", "long", offset=100000)), "long")

    def test_fix_all_is_order_independent(self):
        text = "i beleive teh answer"
        forward = [
            _issue("beleive", "believe strongly in", offset=2),
            _issue("teh", "the", offset=10),
        ]
        reverse = list(reversed(forward))
        self.assertEqual(fix_all(text, forward), "i believe strongly in the answer")
        self.assertEqual(fix_all(text, forward), fix_all(text, reverse))

    def test_fix_all_skips_issues_removed_by_an_overlapping_fix(self):
        issues = [_issue("very very bad", "excellent"), _issue("bad", "good")]
        self.assertEqual(fix_all("very very bad", issues), "excellent")

    def test_remaining_issues_drops_the_ones_no_longer_present(self):
        issues = [_issue("quick", "slow"), _issue("missing", "x")]
        remaining = remaining_issues("the quick fox", issues)
        self.assertEqual([i.original for i in remaining], ["quick"])

    def test_unicode_suggestion_applies(self):
        self.assertEqual(apply_fix("toi dang viet ma", _issue("toi", "tôi")),
                         "tôi dang viet ma")


class GrammarModelTest(unittest.TestCase):
    def test_issue_type_maps_from_wire(self):
        self.assertIs(GrammarIssueType.from_wire("spelling"), GrammarIssueType.SPELLING)
        self.assertIs(GrammarIssueType.from_wire("SPELLING"), GrammarIssueType.SPELLING)
        self.assertIs(GrammarIssueType.from_wire("style"), GrammarIssueType.STYLE)
        self.assertIs(GrammarIssueType.from_wire("something-new"), GrammarIssueType.OTHER)
        self.assertIs(GrammarIssueType.from_wire(None), GrammarIssueType.OTHER)

    def test_issue_type_round_trips(self):
        for member in GrammarIssueType:
            self.assertIs(GrammarIssueType.from_wire(member.wire_value), member)

    def test_colours_match_macos_and_windows(self):
        self.assertEqual(GrammarIssueType.SPELLING.tint_color, "#ef4444")
        self.assertEqual(GrammarIssueType.GRAMMAR.tint_color, "#f59e0b")
        self.assertEqual(GrammarIssueType.STYLE.tint_color, "#5d87ff")

    def test_result_from_wire_parses_issues(self):
        result = GrammarResult.from_wire(
            {"score": 82, "issues": [
                {"type": "spelling", "original": "teh", "suggestion": "the",
                 "offset": 4, "reason": "typo"},
            ]}
        )
        self.assertIsNotNone(result)
        self.assertEqual(result.score, 82)
        self.assertEqual(len(result.issues), 1)
        self.assertIs(result.issues[0].issue_type, GrammarIssueType.SPELLING)
        # length defaults to len(original) when the backend omits it.
        self.assertEqual(result.issues[0].length, 3)

    def test_result_from_wire_handles_absent_payload(self):
        self.assertIsNone(GrammarResult.from_wire(None))
        self.assertIsNone(GrammarResult.from_wire({}))

    def test_issue_from_wire_tolerates_missing_fields(self):
        issue = GrammarIssue.from_wire({})
        self.assertEqual(issue.original, "")
        self.assertEqual(issue.suggestion, "")
        self.assertIs(issue.issue_type, GrammarIssueType.OTHER)


if __name__ == "__main__":
    unittest.main(verbosity=2)
