import XCTest
@testable import DraftRight

/// Grammar fixes resolve by CONTENT, never by the LLM's offset. Trusting
/// offsets spliced suggestions into the middle of words
/// ("showswincorrectlyectly", BR#49). Mirrors
/// DraftRightWindows.PureTests/GrammarFixerTests.cs.
final class GrammarFixTests: XCTestCase {

    private func issue(_ original: String, _ suggestion: String,
                       offset: Int = 0, type: String = "grammar") -> GrammarIssue {
        GrammarIssue(type: type, offset: offset, length: original.count,
                     original: original, suggestion: suggestion, reason: "test")
    }

    private func apply(_ text: String, _ i: GrammarIssue) -> String {
        guard let r = GrammarFix.resolveRange(of: i, in: text) else { return text }
        var out = text
        out.replaceSubrange(r, with: i.suggestion)
        return out
    }

    func testResolvesByContentWhenOffsetIsWrong() {
        let text = "This sentence shows incorrectly."
        let fixed = apply(text, issue("incorrectly", "correctly", offset: 9999))
        XCTAssertEqual(fixed, "This sentence shows correctly.")
        // The exact splice-into-the-middle-of-a-word failure from BR#49.
        XCTAssertFalse(fixed.contains("showswincorrectlyectly"))
    }

    func testOffsetDisambiguatesDuplicatesNearestWins() {
        let text = "cat dog cat dog cat"          // "cat" at 0, 8, 16
        XCTAssertEqual(apply(text, issue("cat", "fox", offset: 15)), "cat dog cat dog fox")
        XCTAssertEqual(apply(text, issue("cat", "fox", offset: 0)),  "fox dog cat dog cat")
    }

    func testTieOnDistancePrefersEarlierOccurrence() {
        // "ab" at 0 and 4; claimed 2 is equidistant. `min` is stable → earliest.
        XCTAssertEqual(apply("ab..ab", issue("ab", "XY", offset: 2)), "XY..ab")
    }

    func testStaleIssueLeavesTextUnchanged() {
        XCTAssertEqual(apply("already fixed", issue("nonexistent", "whatever")), "already fixed")
    }

    func testEmptyOriginalIsNeverResolved() {
        XCTAssertNil(GrammarFix.resolveRange(of: issue("", "x"), in: "some text"))
    }

    func testOffsetBeyondTextLengthIsClamped() {
        XCTAssertEqual(apply("short", issue("short", "long", offset: 100_000)), "long")
    }

    /// A lengthening fix early in the string invalidates every later offset.
    /// Content resolution is what makes Fix All order-independent.
    func testFixAllIsOrderIndependent() {
        let text = "i beleive teh answer"
        let a = issue("beleive", "believe strongly in", offset: 2)
        let b = issue("teh", "the", offset: 10)
        XCTAssertEqual(apply(apply(text, a), b), "i believe strongly in the answer")
        XCTAssertEqual(apply(apply(text, a), b), apply(apply(text, b), a))
    }

    func testUnicodeSuggestionApplies() {
        XCTAssertEqual(apply("toi dang viet ma", issue("toi", "tôi")), "tôi dang viet ma")
    }

    func testIssueColoursMatchTheOtherClients() {
        // Windows GrammarIssueType.HexColor / Linux tint_color use the same
        // mapping: spelling red, grammar orange, style blue.
        XCTAssertEqual(issue("x", "y", type: "spelling").color, .systemRed)
        XCTAssertEqual(issue("x", "y", type: "grammar").color, .systemOrange)
        XCTAssertEqual(issue("x", "y", type: "style").color, .systemBlue)
    }
}
