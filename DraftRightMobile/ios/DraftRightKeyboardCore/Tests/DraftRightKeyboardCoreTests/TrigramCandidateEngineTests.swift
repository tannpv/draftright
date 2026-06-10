import XCTest
@testable import DraftRightKeyboardCore

final class TrigramCandidateEngineTests: XCTestCase {

    private func makeViEngine() -> TrigramCandidateEngine {
        TrigramCandidateEngine(wordList: InMemoryWordList(
            words: [
                ("người", 100), ("ngoại", 80), ("ngon", 60), ("ngạc", 40), ("ngân", 30),
            ],
            bigrams: [
                "người": ["đẹp": 20, "tốt": 15, "yêu": 10],
            ]
        ))
    }

    func testPrefixCompletionOrderedByFrequency() {
        let out = makeViEngine().suggest(composing: "ng", previousTokens: [], limit: 3)
        XCTAssertEqual(out.map { $0.text }, ["người", "ngoại", "ngon"])
    }

    func testEmptyComposingReturnsNextWord() {
        let out = makeViEngine().suggest(composing: "", previousTokens: ["người"], limit: 5)
        XCTAssertEqual(out.map { $0.text }, ["đẹp", "tốt", "yêu"])
    }

    func testBigramBoostReranksCompletions() {
        let engine = TrigramCandidateEngine(wordList: InMemoryWordList(
            words: [("nhanh", 50), ("nóng", 50)],
            bigrams: ["người": ["nóng": 5]]
        ))
        let out = engine.suggest(composing: "n", previousTokens: ["người"], limit: 2)
        XCTAssertEqual(out.first?.text, "nóng")
    }

    func testUnknownPrefixIsEmpty() {
        XCTAssertTrue(makeViEngine().suggest(composing: "zzzz", previousTokens: [], limit: 7).isEmpty)
    }

    func testLimitClamps() {
        XCTAssertEqual(makeViEngine().suggest(composing: "ng", previousTokens: [], limit: 2).count, 2)
    }

    func testCaseInsensitivePreservesOriginalCasing() {
        let engine = TrigramCandidateEngine(wordList: InMemoryWordList(words: [("Saigon", 10)]))
        XCTAssertEqual(engine.suggest(composing: "sai", previousTokens: [], limit: 1).first?.text, "Saigon")
    }
}
