import XCTest
@testable import DraftRightKeyboardCore

private final class UpperComposer: BufferingComposer {
    override func transform(_ raw: String) -> String { raw.uppercased() }
}

final class BufferingComposerTests: XCTestCase {

    func testAccumulatesAndTransforms() {
        let c = UpperComposer()
        _ = c.onKey("a"); _ = c.onKey("b")
        XCTAssertEqual(c.currentComposingText(), "AB")
    }

    func testLetterReturnsComposing() {
        XCTAssertEqual(UpperComposer().onKey("a"), .composing("A"))
    }

    func testNonLetterCommitsBufferPlusChar() {
        let c = UpperComposer()
        _ = c.onKey("a"); _ = c.onKey("b")
        XCTAssertEqual(c.onKey(" "), .commit("AB "))
        XCTAssertEqual(c.currentComposingText(), "")
    }

    func testNonLetterEmptyPassesThrough() {
        XCTAssertEqual(UpperComposer().onKey(" "), .passThrough)
    }

    func testBackspaceConsumesWhenEmpty() {
        let c = UpperComposer()
        _ = c.onKey("a")
        XCTAssertEqual(c.onBackspace(), .consumed)
    }
}

final class PassthroughComposerTests: XCTestCase {
    func testKeysPassThrough() {
        let c = PassthroughComposer()
        XCTAssertEqual(c.onKey("a"), .passThrough)
        XCTAssertEqual(c.onBackspace(), .passThrough)
        XCTAssertEqual(c.currentComposingText(), "")
    }
}
