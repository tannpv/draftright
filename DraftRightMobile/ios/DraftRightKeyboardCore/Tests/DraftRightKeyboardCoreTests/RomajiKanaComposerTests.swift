import XCTest
@testable import DraftRightKeyboardCore

final class RomajiKanaComposerTests: XCTestCase {

    @discardableResult
    private func typeAll(_ c: RomajiKanaComposer, _ s: String) -> ComposeResult {
        var r: ComposeResult = .passThrough
        for ch in s { r = c.onKey(ch) }
        return r
    }

    func testWatashiComposesKana() {
        let c = RomajiKanaComposer()
        typeAll(c, "watashi")
        XCTAssertEqual(c.currentComposingText(), "わたし")
    }

    func testNihongoComposesKana() {
        let c = RomajiKanaComposer()
        typeAll(c, "nihongo")
        XCTAssertEqual(c.currentComposingText(), "にほんご")
    }

    func testNonLetterCommitsKanaPlusChar() {
        let c = RomajiKanaComposer()
        typeAll(c, "watashi")
        let r = c.onKey(" ")
        XCTAssertEqual(r, .commit("わたし "))
        XCTAssertEqual(c.currentComposingText(), "")
    }

    func testBackspaceStripsOneChar() {
        let c = RomajiKanaComposer()
        typeAll(c, "wa")
        XCTAssertEqual(c.currentComposingText(), "わ")
        _ = c.onBackspace()
        XCTAssertEqual(c.currentComposingText(), "w")
    }

    func testNonLetterEmptyBufferPassesThrough() {
        XCTAssertEqual(RomajiKanaComposer().onKey(" "), .passThrough)
    }
}
