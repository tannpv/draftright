import XCTest
@testable import DraftRight

/// Word-level LCS diff. Cases mirror DraftRightWindows.PureTests/WordDiffTests.cs
/// and DraftRightLinux/test/test_diff_and_grammar.py so a divergence between the
/// three implementations shows up as a failure rather than as different
/// behaviour on screen (#107 established they are byte-identical).
final class WordDiffTests: XCTestCase {

    private func rebuild(_ tokens: [DiffToken]) -> String {
        tokens.map(\.text).joined()
    }

    private func kinds(_ tokens: [DiffToken]) -> [String] {
        tokens.map { t in
            switch t.kind {
            case .equal: return "equal"
            case .deleted: return "deleted"
            case .inserted: return "inserted"
            }
        }
    }

    // MARK: - #145 regression

    /// Before the fix, `longestCommonSubsequence` built its table with
    /// `for i in 1...m`; `1...0` is an invalid ClosedRange and TRAPS, so any
    /// diff with an empty side crashed the app outright. Reachable in
    /// production via an empty `rewritten_text` from the backend.
    func testEmptySideDoesNotCrash() {
        for (old, new) in [("", "hello"), ("hello", ""), ("", "")] {
            let (o, n) = WordDiff.diff(old: old, new: new)
            XCTAssertEqual(rebuild(o), old, "old side lost content for (\(old),\(new))")
            XCTAssertEqual(rebuild(n), new, "new side lost content for (\(old),\(new))")
        }
    }

    func testEmptyOldSideMarksEverythingInserted() {
        let (o, n) = WordDiff.diff(old: "", new: "brand new")
        XCTAssertTrue(o.isEmpty)
        XCTAssertEqual(Set(kinds(n)), ["inserted"])
    }

    // MARK: - Core behaviour

    func testIdenticalTextIsAllEqual() {
        let (o, n) = WordDiff.diff(old: "the quick fox", new: "the quick fox")
        XCTAssertEqual(Set(kinds(o)), ["equal"])
        XCTAssertEqual(Set(kinds(n)), ["equal"])
    }

    func testReplacedWordIsDeletedLeftInsertedRight() {
        let (o, n) = WordDiff.diff(old: "the quick fox", new: "the slow fox")
        XCTAssertTrue(o.contains { $0.text == "quick" && $0.kind == .deleted })
        XCTAssertTrue(n.contains { $0.text == "slow" && $0.kind == .inserted })
    }

    func testPureInsertionLeavesLeftUntouched() {
        let (o, n) = WordDiff.diff(old: "hello world", new: "hello brave world")
        XCTAssertFalse(o.contains { $0.kind == .deleted })
        XCTAssertTrue(n.contains { $0.text == "brave" && $0.kind == .inserted })
    }

    func testPureDeletionLeavesRightUntouched() {
        let (o, n) = WordDiff.diff(old: "hello brave world", new: "hello world")
        XCTAssertTrue(o.contains { $0.text == "brave" && $0.kind == .deleted })
        XCTAssertFalse(n.contains { $0.kind == .inserted })
    }

    /// Whitespace is tokenised precisely so this holds — the diff view renders
    /// these runs directly, so any loss shows up as mangled spacing on screen.
    func testTokensRebuildTheOriginalStringsExactly() {
        let a = "the  quick\nbrown fox"
        let b = "the slow\nbrown  fox"
        let (o, n) = WordDiff.diff(old: a, new: b)
        XCTAssertEqual(rebuild(o), a)
        XCTAssertEqual(rebuild(n), b)
    }

    func testCompletelyDifferentTextSharesNothing() {
        let (o, n) = WordDiff.diff(old: "aaa", new: "bbb")
        XCTAssertEqual(Set(kinds(o)), ["deleted"])
        XCTAssertEqual(Set(kinds(n)), ["inserted"])
    }

    func testRepeatedWordsDoNotLoseContent() {
        let a = "a a a b", b = "a b b"
        let (o, n) = WordDiff.diff(old: a, new: b)
        XCTAssertEqual(rebuild(o), a)
        XCTAssertEqual(rebuild(n), b)
    }

    /// Vietnamese is a first-class case for this product.
    func testUnicodeRoundTrips() {
        let a = "tôi đang viết mã", b = "tôi đang viết code"
        let (o, n) = WordDiff.diff(old: a, new: b)
        XCTAssertEqual(rebuild(o), a)
        XCTAssertEqual(rebuild(n), b)
        XCTAssertTrue(o.contains { $0.text == "mã" && $0.kind == .deleted })
        XCTAssertTrue(n.contains { $0.text == "code" && $0.kind == .inserted })
    }
}
