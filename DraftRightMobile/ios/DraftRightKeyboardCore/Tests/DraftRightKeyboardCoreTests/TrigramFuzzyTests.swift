import XCTest
@testable import DraftRightKeyboardCore

/// Typo tolerance (#207 gap #3). Mirrors the Android `TrigramFuzzyTest` so the
/// two platforms' engines stay 1:1 — when the exact prefix scan leaves empty
/// slots, they're topped up with edit-distance-1 dictionary words; exact
/// matches are never displaced.
final class TrigramFuzzyTests: XCTestCase {

    private func engine() -> TrigramCandidateEngine {
        TrigramCandidateEngine(wordList: InMemoryWordList(
            words: [("người", 100), ("ngon", 60), ("việt", 50), ("nhà", 40)]
        ))
    }

    private func texts(_ term: String, _ limit: Int = 5) -> [String] {
        engine().suggest(composing: term, previousTokens: [], limit: limit).map { $0.text }
    }

    func testSubstitutionTypoSurfacesCorrection() {
        XCTAssertTrue(texts("nqon").contains("ngon"))
    }

    func testWrongToneTypoSurfacesIntendedWord() {
        XCTAssertTrue(texts("ngưới").contains("người"))
    }

    func testMissingCharTypoSurfacesCorrection() {
        XCTAssertTrue(texts("nhs").contains("nhà"))
    }

    func testExactPrefixNotDisplacedByFuzzy() {
        XCTAssertEqual(texts("ng", 2), ["người", "ngon"])
    }

    func testFarTypoSurfacesNothing() {
        XCTAssertTrue(texts("zzzz").isEmpty)
    }
}
