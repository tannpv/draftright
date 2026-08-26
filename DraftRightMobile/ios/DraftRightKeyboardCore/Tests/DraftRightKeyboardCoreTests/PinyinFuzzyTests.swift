import XCTest
@testable import DraftRightKeyboardCore

/// Fuzzy-pinyin folding (#211). Mirrors the Kotlin PinyinFuzzyTest.
final class PinyinFuzzyTests: XCTestCase {

    func testRetroflexInitialsFold() {
        XCTAssertEqual(PinyinFuzzy.fold("shi"), "si")
        XCTAssertEqual(PinyinFuzzy.fold("zhongguo"), "zongguo")
        XCTAssertEqual(PinyinFuzzy.fold("chang"), "can")
    }

    func testNasalFinalsFold() {
        XCTAssertEqual(PinyinFuzzy.fold("jing"), "jin")
        XCTAssertEqual(PinyinFuzzy.fold("bang"), "ban")
        XCTAssertEqual(PinyinFuzzy.fold("ming"), "min")
    }

    func testPlainUnchanged() {
        XCTAssertEqual(PinyinFuzzy.fold("ni"), "ni")
        XCTAssertEqual(PinyinFuzzy.fold("hao"), "hao")
    }
}
