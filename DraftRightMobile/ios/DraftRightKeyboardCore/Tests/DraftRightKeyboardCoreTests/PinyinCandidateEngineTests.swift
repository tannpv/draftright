import XCTest
@testable import DraftRightKeyboardCore

/// Sentence-level pinyin candidates (#211). Mirrors the Kotlin
/// PinyinCandidateEngineTest.
final class PinyinCandidateEngineTests: XCTestCase {

    private let dict: [String: [String]] = [
        "ni": ["你"], "hao": ["好"], "nihao": ["你好"],
        "wo": ["我"], "shi": ["是", "时"], "men": ["们"],
        "women": ["我们"], "beijing": ["北京"],
    ]
    private var engine: PinyinCandidateEngine { PinyinCandidateEngine(dictionary: dict) }

    private func texts(_ s: String) -> [String] {
        engine.suggest(composing: s, previousTokens: [], limit: 7).map { $0.text }
    }

    func testRunTogetherPinyinBuildsSegmentedCandidate() {
        XCTAssertTrue(texts("woshi").contains("我是"))
    }

    func testThreeSyllables() {
        XCTAssertTrue(texts("womenshi").contains("我们是"))
    }

    func testRawPinyinFallbackOffered() {
        XCTAssertTrue(texts("woshi").contains("woshi"))
    }

    func testExactWordRanksFirst() {
        XCTAssertEqual(texts("nihao").first, "你好")
    }

    func testSingleSyllableUnchanged() {
        XCTAssertEqual(texts("wo").filter { $0 != "wo" }, ["我"])
    }

    func testUnsegmentableFallsBack() {
        XCTAssertEqual(texts("xyz"), ["xyz"])
    }

    func testInitialsAbbreviationCommitsWholeWord() {
        XCTAssertTrue(texts("nh").contains("你好"))
        XCTAssertTrue(texts("bj").contains("北京"))
    }

    func testAbbreviationOfTwoSyllableWord() {
        XCTAssertTrue(texts("wm").contains("我们"))
    }
}
