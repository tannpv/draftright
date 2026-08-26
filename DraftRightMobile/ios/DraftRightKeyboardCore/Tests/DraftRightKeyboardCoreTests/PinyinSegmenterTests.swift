import XCTest
@testable import DraftRightKeyboardCore

/// Sentence-level pinyin segmentation (#211). Mirrors the Kotlin
/// PinyinSegmenterTest 1:1.
final class PinyinSegmenterTests: XCTestCase {

    func testTwoSyllableWord() {
        XCTAssertEqual(PinyinSegmenter.segment("woshi"), ["wo", "shi"])
        XCTAssertEqual(PinyinSegmenter.segment("nihao"), ["ni", "hao"])
    }

    func testPrefersLongestSyllable() {
        XCTAssertEqual(PinyinSegmenter.segment("xian"), ["xian"])
        XCTAssertEqual(PinyinSegmenter.segment("xianggang"), ["xiang", "gang"])
    }

    func testMultiSyllableSentence() {
        XCTAssertEqual(PinyinSegmenter.segment("womenshi"), ["wo", "men", "shi"])
        XCTAssertEqual(PinyinSegmenter.segment("zhongguoren"), ["zhong", "guo", "ren"])
    }

    func testSingleSyllable() {
        XCTAssertEqual(PinyinSegmenter.segment("hao"), ["hao"])
    }

    func testInvalidReturnsNil() {
        XCTAssertNil(PinyinSegmenter.segment("xyz"))
        XCTAssertNil(PinyinSegmenter.segment("woxq"))
    }

    func testEmpty() {
        XCTAssertEqual(PinyinSegmenter.segment(""), [])
    }

    func testBacktracks() {
        XCTAssertEqual(PinyinSegmenter.segment("xier"), ["xi", "er"])
    }
}
