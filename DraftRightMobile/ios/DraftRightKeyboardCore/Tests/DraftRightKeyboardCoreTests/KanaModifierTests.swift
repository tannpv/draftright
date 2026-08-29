import XCTest
@testable import DraftRightKeyboardCore

/// Kana dakuten/handakuten/small cycling (#212 phase 3). Mirrors Kotlin KanaModifierTest.
final class KanaModifierTests: XCTestCase {

    func testDakutenToggles() {
        XCTAssertEqual(KanaModifier.cycle("か"), "が")
        XCTAssertEqual(KanaModifier.cycle("が"), "か")
        XCTAssertEqual(KanaModifier.cycle("し"), "じ")
        XCTAssertEqual(KanaModifier.cycle("た"), "だ")
    }

    func testHRowCycle() {
        XCTAssertEqual(KanaModifier.cycle("は"), "ば")
        XCTAssertEqual(KanaModifier.cycle("ば"), "ぱ")
        XCTAssertEqual(KanaModifier.cycle("ぱ"), "は")
    }

    func testTsu() {
        XCTAssertEqual(KanaModifier.cycle("つ"), "っ")
        XCTAssertEqual(KanaModifier.cycle("っ"), "づ")
        XCTAssertEqual(KanaModifier.cycle("づ"), "つ")
    }

    func testSmall() {
        XCTAssertEqual(KanaModifier.cycle("や"), "ゃ")
        XCTAssertEqual(KanaModifier.cycle("あ"), "ぁ")
    }

    func testNoVariantUnchanged() {
        XCTAssertEqual(KanaModifier.cycle("な"), "な")
        XCTAssertEqual(KanaModifier.cycle("ん"), "ん")
        XCTAssertEqual(KanaModifier.cycle("ら"), "ら")
    }
}
