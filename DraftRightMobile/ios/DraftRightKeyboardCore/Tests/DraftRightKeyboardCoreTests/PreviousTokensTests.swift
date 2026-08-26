import XCTest
@testable import DraftRightKeyboardCore

/// N-gram context extraction. Mirrors Android `PreviousTokensTest` 1:1 so both
/// platforms feed the engine the same previous tokens.
final class PreviousTokensTests: XCTestCase {

    func testExtractsCommittedWordsOrderPreserved() {
        XCTAssertEqual(PreviousTokens.fromTextBeforeCursor("xin chào ", composing: ""), ["xin", "chào"])
    }

    func testExcludesLiveComposingWord() {
        XCTAssertEqual(
            PreviousTokens.fromTextBeforeCursor("tôi là sinh viên", composing: "viên"),
            ["tôi", "là", "sinh"]
        )
    }

    func testCasePreserved() {
        XCTAssertEqual(PreviousTokens.fromTextBeforeCursor("Xin Chào ", composing: ""), ["Xin", "Chào"])
    }

    func testDepthCappedAtMax() {
        XCTAssertEqual(PreviousTokens.fromTextBeforeCursor("a b c d e ", composing: ""), ["c", "d", "e"])
    }

    func testTrailingPunctuationTrimmed() {
        XCTAssertEqual(PreviousTokens.fromTextBeforeCursor("Chào,", composing: ""), ["Chào"])
    }

    func testEmptyAndWhitespaceYieldNothing() {
        XCTAssertEqual(PreviousTokens.fromTextBeforeCursor("", composing: ""), [])
        XCTAssertEqual(PreviousTokens.fromTextBeforeCursor("   ", composing: ""), [])
        XCTAssertEqual(PreviousTokens.fromTextBeforeCursor(nil, composing: ""), [])
    }

    func testComposingNotSuffixIsNotStripped() {
        XCTAssertEqual(PreviousTokens.fromTextBeforeCursor("bún bò", composing: "phở"), ["bún", "bò"])
    }
}
