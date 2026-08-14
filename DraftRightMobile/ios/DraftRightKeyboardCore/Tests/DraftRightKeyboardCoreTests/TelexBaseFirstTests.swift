import XCTest
@testable import DraftRightKeyboardCore

/// Base-first Telex: type all the base letters, then the marks at the end (#152).
///
/// The iOS mirror of Android's `TelexBaseFirstTest`. This already worked for
/// syllables ending in a CONSONANT ("canaf" -> cần); it failed the moment the
/// syllable ended in a vowel, because the lookback stopped at the first
/// vowel-like char it met — for "day" that is the 'y', which is not the letter
/// typed, so the modifier was inserted as a literal and a following tone landed
/// on the wrong vowel ("dayaj" -> "daỵa" instead of "dậy").
///
/// Kept in lock-step with the Android cases so both composers stay one logic in
/// two languages.
final class TelexBaseFirstTests: XCTestCase {

    private func type(_ keys: String) -> String {
        let c = TelexComposer()
        var last: ComposeResult = .passThrough
        for ch in keys { last = c.onKey(ch) }
        switch last {
        case .composing(let s), .commit(let s): return s
        default: return c.currentComposingText()
        }
    }

    // ── The reported cases ────────────────────────────────────────────────

    func test_dayaj_composes_dậy_the_reported_case() { XCTAssertEqual(type("dayaj"), "dậy") }
    func test_daya_composes_dây_circumflex_after_trailing_vowel() { XCTAssertEqual(type("daya"), "dây") }
    func test_mayas_composes_mấy() { XCTAssertEqual(type("mayas"), "mấy") }
    func test_tayas_composes_tấy() { XCTAssertEqual(type("tayas"), "tấy") }
    func test_caya_composes_cây() { XCTAssertEqual(type("caya"), "cây") }

    // ── Inline typing must be unaffected ──────────────────────────────────

    func test_daayj_still_composes_dậy_inline() { XCTAssertEqual(type("daayj"), "dậy") }
    func test_maays_still_composes_mấy_inline() { XCTAssertEqual(type("maays"), "mấy") }
    func test_toois_still_composes_tối() { XCTAssertEqual(type("toois"), "tối") }

    // ── Base-first past consonants keeps working ──────────────────────────

    func test_canaf_still_composes_cần() { XCTAssertEqual(type("canaf"), "cần") }
    func test_nguyenex_still_composes_nguyễn() { XCTAssertEqual(type("nguyenex"), "nguyễn") }
    func test_truongwf_still_composes_trường() { XCTAssertEqual(type("truongwf"), "trường") }
    func test_vietej_still_composes_việt() { XCTAssertEqual(type("vietej"), "việt") }

    // ── The guard: a modifier must NOT reach past another nucleus vowel ───
    //
    // "oeo" (ngoẻo) and "oao" are real three-vowel clusters whose final letter
    // is a literal vowel, not a modifier for the first one. Letting the scan
    // cross a nucleus turned these into "ôe"/"ôa" — 312 corpus regressions on
    // the first attempt at this fix on Android.

    func test_oeo_stays_literal_o_is_a_nucleus_not_a_modifier() { XCTAssertEqual(type("oeo"), "oeo") }
    func test_oao_stays_literal() { XCTAssertEqual(type("oao"), "oao") }
    func test_boeo_stays_literal() { XCTAssertEqual(type("boeo"), "boeo") }
    func test_coaos_stays_literal_with_its_tone() { XCTAssertEqual(type("coaos"), "coáo") }

    // ── Cancel-by-retype still works through the same path ────────────────

    func test_aa_gives_â_and_aaa_cancels_back() {
        XCTAssertEqual(type("aa"), "â")
        XCTAssertEqual(type("aaa"), "aa")
    }

    func test_oo_gives_ô_and_ooo_cancels_back() {
        XCTAssertEqual(type("oo"), "ô")
        XCTAssertEqual(type("ooo"), "oo")
    }

    func test_hoaf_composes_hòa_two_vowel_cluster_with_a_tone() { XCTAssertEqual(type("hoaf"), "hòa") }
    func test_loan_is_untouched_when_no_modifier_follows() { XCTAssertEqual(type("loan"), "loan") }
}
