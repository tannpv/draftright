import XCTest
@testable import DraftRightKeyboardCore

/// The keyboard asks its candidate engine for a correction rather than holding
/// a second copy of the dictionary (#207) — mirror of Android
/// `CandidateEngineAutoCorrectTest`.
final class CandidateEngineAutoCorrectTests: XCTestCase {

    private let words = InMemoryWordList(words: [("không", 668048), ("khô", 4000)])

    func testTrigramEngineDelegatesToAutoCorrector() {
        let engine = TrigramCandidateEngine(wordList: words)
        XCTAssertEqual(engine.autoCorrect("khôg"), AutoCorrector.correct("khôg", words))
        XCTAssertEqual(engine.autoCorrect("khôg"), "không")
    }

    func testEngineWithoutADictionaryNeverCorrects() {
        struct Stub: CandidateEngine {
            func suggest(composing: String, previousTokens: [String], limit: Int) -> [Candidate] { [] }
        }
        XCTAssertNil(Stub().autoCorrect("khôg"))
    }
}
