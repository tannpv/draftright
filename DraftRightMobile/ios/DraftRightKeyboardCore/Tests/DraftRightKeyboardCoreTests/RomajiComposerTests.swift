import XCTest
@testable import DraftRightKeyboardCore

final class RomajiComposerTests: XCTestCase {
    private func kana(_ romaji: String) -> String {
        let c = RomajiComposer()
        return c.feed(romaji)
    }

    func test_basic_words() {
        XCTAssertEqual(kana("nihongo"), "にほんご")
        XCTAssertEqual(kana("sushi"), "すし")
        XCTAssertEqual(kana("tokyo"), "ときょ")       // to + kyo
        XCTAssertEqual(kana("konnichiwa"), "こんにちわ") // ko nn(ん) ni chi wa
    }

    func test_irregulars() {
        XCTAssertEqual(kana("shi"), "し")
        XCTAssertEqual(kana("chi"), "ち")
        XCTAssertEqual(kana("tsu"), "つ")
        XCTAssertEqual(kana("fu"), "ふ")
        XCTAssertEqual(kana("ji"), "じ")
    }

    func test_y_glides() {
        XCTAssertEqual(kana("kya"), "きゃ")
        XCTAssertEqual(kana("sha"), "しゃ")
        XCTAssertEqual(kana("cho"), "ちょ")
        XCTAssertEqual(kana("ryu"), "りゅ")
    }

    func test_sokuon_doubled_consonant() {
        XCTAssertEqual(kana("kitto"), "きっと")  // ki + っ + to
        XCTAssertEqual(kana("kippu"), "きっぷ")  // ki + っ + pu
    }

    func test_moraic_n() {
        XCTAssertEqual(kana("nn"), "ん")    // doubled n → single ん
        XCTAssertEqual(kana("n'"), "ん")    // apostrophe forces a standalone ん
        XCTAssertEqual(kana("hon"), "ほん")        // ho + trailing n finalizes to ん
        XCTAssertEqual(kana("honda"), "ほんだ")    // ho + n(+consonant→ん) + da
    }

    /// A trailing lone "n" finalizes to ん so the kana is dictionary-lookable
    /// ("nihon" → にほん → 日本). Previously it stayed literal ("にほn"), so the
    /// candidate engine never matched and only hiragana showed.
    func test_trailing_moraic_n_finalized() {
        XCTAssertEqual(kana("nihon"), "にほん")
        XCTAssertEqual(kana("hon"), "ほん")
        XCTAssertEqual(kana("n"), "ん")
        XCTAssertEqual(kana("nn"), "ん")
        XCTAssertEqual(kana("nihongo"), "にほんご")   // mid-word n still binds
        XCTAssertEqual(kana("konnichiwa"), "こんにちわ")
    }

    func test_pending_tail_is_shown_as_romaji() {
        let c = RomajiComposer()
        XCTAssertEqual(c.feed("k"), "k")       // waiting
        XCTAssertEqual(c.feed("y"), "ky")      // still waiting (prefix of kya)
        XCTAssertEqual(c.feed("a"), "きゃ")     // resolves
    }

    func test_lone_trailing_n_waits_then_commits_with_apostrophe() {
        let c = RomajiComposer()
        XCTAssertEqual(c.feed("ho"), "ほ")
        XCTAssertEqual(c.feed("n"), "ほん")     // trailing n finalizes to ん
        XCTAssertEqual(c.feed("'"), "ほん")     // apostrophe keeps the moraic ん
    }

    func test_reset() {
        let c = RomajiComposer()
        c.feed("nihon")
        c.reset()
        XCTAssertEqual(c.text(), "")
        XCTAssertEqual(c.feed("a"), "あ")
    }
}
