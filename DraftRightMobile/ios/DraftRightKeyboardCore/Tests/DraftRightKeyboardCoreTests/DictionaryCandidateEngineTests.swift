import XCTest
@testable import DraftRightKeyboardCore

final class DictionaryCandidateEngineTests: XCTestCase {

    private let kana = ["にほんご": ["日本語"], "かんじ": ["漢字", "幹事"]]
    private let pinyin = ["nihao": ["你好"], "zhongwen": ["中文", "中問"]]

    func testKanaReadingReturnsKanjiPlusFallback() {
        let c = DictionaryCandidateEngine(dictionary: kana).suggest(composing: "にほんご")
        XCTAssertEqual(c.first?.text, "日本語")
        XCTAssertTrue(c.contains { $0.text == "にほんご" })
    }

    func testPinyinReadingReturnsHanzi() {
        let c = DictionaryCandidateEngine(dictionary: pinyin).suggest(composing: "zhongwen")
        XCTAssertEqual(Array(c.prefix(2)).map { $0.text }, ["中文", "中問"])
    }

    func testUnknownReadingOffersReadingItself() {
        let c = DictionaryCandidateEngine(dictionary: kana).suggest(composing: "sora")
        XCTAssertEqual(c.map { $0.text }, ["sora"])
    }

    func testEmptyInput() {
        XCTAssertTrue(DictionaryCandidateEngine(dictionary: kana).suggest(composing: "").isEmpty)
    }
}
