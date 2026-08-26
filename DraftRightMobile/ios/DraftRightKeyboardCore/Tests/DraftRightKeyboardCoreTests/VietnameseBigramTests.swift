import XCTest
@testable import DraftRightKeyboardCore

/// Next-word prediction from the bundled Vietnamese bigrams (#207 Phase 2).
/// These (prev → top successor) expectations are the **agreement guard** between
/// the Swift `VietnameseBootstrapWordList.bigrams` and Android's
/// `res/raw/wordlist_vi_bigrams.tsv` — the two copies must produce the same top
/// successor for each of these readings, or one has drifted (RULE #1).
final class VietnameseBigramTests: XCTestCase {

    private func top(_ previous: String) -> String? {
        TrigramCandidateEngine(wordList: VietnameseBootstrapWordList.wordList)
            .suggest(composing: "", previousTokens: [previous], limit: 1)
            .first?.text
    }

    func testCanonicalCollocationsTopSuccessor() {
        XCTAssertEqual(top("xin"), "chào")
        XCTAssertEqual(top("cảm"), "ơn")
        XCTAssertEqual(top("việt"), "nam")
        XCTAssertEqual(top("năm"), "mới")
        XCTAssertEqual(top("buổi"), "sáng")
        XCTAssertEqual(top("hôm"), "nay")
        XCTAssertEqual(top("gặp"), "lại")
        XCTAssertEqual(top("tạm"), "biệt")
    }

    func testUnknownReadingHasNoNextWord() {
        XCTAssertNil(top("zzz"))
    }
}
