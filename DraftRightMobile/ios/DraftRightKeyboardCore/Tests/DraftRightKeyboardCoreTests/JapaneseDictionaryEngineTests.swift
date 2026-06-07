import XCTest
@testable import DraftRightKeyboardCore

/// Spike proof (Task 1, engine pivot): kana→kanji conversion runs through the
/// existing CandidateEngine seam + RomajiComposer with NO native code — a
/// downloadable reading→kanji dictionary is all the engine needs. Demonstrates
/// the lighter alternative to bridging librime into the iOS extension.
final class JapaneseDictionaryEngineTests: XCTestCase {

    private let dict: [String: [String]] = [
        "にほん": ["日本"],
        "にほんご": ["日本語"],
        "かんじ": ["漢字", "幹事"],
    ]

    func testRomajiConvertsToRankedKanji() {
        let e = JapaneseDictionaryEngine(dictionary: dict)
        let c = e.suggest(composing: "nihongo")
        XCTAssertEqual(c.first?.text, "日本語")              // dictionary hit ranked first
        XCTAssertTrue(c.contains { $0.text == "にほんご" })   // plain kana always offered
    }

    func testMultipleKanjiForOneReadingKeepRank() {
        let e = JapaneseDictionaryEngine(dictionary: dict)
        let c = e.suggest(composing: "kanji")
        XCTAssertEqual(Array(c.prefix(2)).map { $0.text }, ["漢字", "幹事"])
    }

    func testUnknownReadingStillOffersKana() {
        let e = JapaneseDictionaryEngine(dictionary: dict)
        let c = e.suggest(composing: "sora")     // not in dict
        XCTAssertEqual(c.map { $0.text }, ["そら"])
    }

    func testEmptyInputNoCandidates() {
        XCTAssertTrue(JapaneseDictionaryEngine(dictionary: dict).suggest(composing: "").isEmpty)
    }
}
