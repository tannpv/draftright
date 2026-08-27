import XCTest
@testable import DraftRightKeyboardCore

/// Hiragana→katakana + katakana candidate (#211/#212). Mirrors Kotlin tests.
final class KatakanaTests: XCTestCase {

    func testConvertsGojuon() {
        XCTAssertEqual(Katakana.fromHiragana("かな"), "カナ")
        XCTAssertEqual(Katakana.fromHiragana("こーひー"), "コーヒー") // ー passes through
        XCTAssertEqual(Katakana.fromHiragana("ゔ"), "ヴ")
    }

    func testSmallAndDakutenConvert() {
        XCTAssertEqual(Katakana.fromHiragana("っ"), "ッ")
        XCTAssertEqual(Katakana.fromHiragana("が"), "ガ")
        XCTAssertEqual(Katakana.fromHiragana("ぱ"), "パ")
        XCTAssertEqual(Katakana.fromHiragana("ゃ"), "ャ")
    }

    func testNonHiraganaPassesThrough() {
        XCTAssertEqual(Katakana.fromHiragana(""), "")
        XCTAssertEqual(Katakana.fromHiragana("abc"), "abc")
        XCTAssertEqual(Katakana.fromHiragana("日本"), "日本")
        XCTAssertEqual(Katakana.fromHiragana("カ"), "カ")
    }

    func testEngineOffersKatakana() {
        let engine = KatakanaCandidateEngine(dictionary: ["かな": ["仮名"]])
        let out = engine.suggest(composing: "かな", previousTokens: [], limit: 10).map { $0.text }
        XCTAssertTrue(out.contains("仮名"))
        XCTAssertTrue(out.contains("カナ"))
        XCTAssertTrue(out.contains("かな"))
        XCTAssertLessThan(out.firstIndex(of: "カナ")!, out.firstIndex(of: "かな")!)
    }

    func testEngineNoDuplicateWhenAlreadyKatakana() {
        let engine = KatakanaCandidateEngine(dictionary: [:])
        XCTAssertEqual(engine.suggest(composing: "カナ", previousTokens: [], limit: 10).map { $0.text }, ["カナ"])
    }
}
