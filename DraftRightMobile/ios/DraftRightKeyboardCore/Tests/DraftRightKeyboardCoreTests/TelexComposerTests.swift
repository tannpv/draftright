import XCTest
@testable import DraftRightKeyboardCore

final class TelexComposerTests: XCTestCase {

    private func finalText(_ keys: Character...) -> String {
        let c = TelexComposer()
        var last: ComposeResult = .passThrough
        for k in keys { last = c.onKey(k) }
        switch last {
        case .composing(let s), .commit(let s): return s
        default: return c.currentComposingText()
        }
    }

    func test_aa_composes_â() { XCTAssertEqual(finalText("a", "a"), "â") }
    func test_oo_composes_ô() { XCTAssertEqual(finalText("o", "o"), "ô") }
    func test_ee_composes_ê() { XCTAssertEqual(finalText("e", "e"), "ê") }
    func test_ow_composes_ơ() { XCTAssertEqual(finalText("o", "w"), "ơ") }
    func test_uw_composes_ư() { XCTAssertEqual(finalText("u", "w"), "ư") }
    func test_aw_composes_ă() { XCTAssertEqual(finalText("a", "w"), "ă") }
    func test_dd_composes_đ() { XCTAssertEqual(finalText("d", "d"), "đ") }
    func test_aaj_composes_ậ() { XCTAssertEqual(finalText("a", "a", "j"), "ậ") }
    func test_uow_composes_ươ() { XCTAssertEqual(finalText("u", "o", "w"), "ươ") }
    func test_uowj_composes_ượ() { XCTAssertEqual(finalText("u", "o", "w", "j"), "ượ") }
    func test_AA_composes_Â() { XCTAssertEqual(finalText("A", "A"), "Â") }

    func test_q_is_direct_commit_no_rule() {
        let c = TelexComposer()
        let r = c.onKey("q")
        if case .composing = r {} else { XCTFail("expected composing") }
    }

    func test_as_composes_á() { XCTAssertEqual(finalText("a", "s"), "á") }
    func test_af_composes_à() { XCTAssertEqual(finalText("a", "f"), "à") }
    func test_ar_composes_ả() { XCTAssertEqual(finalText("a", "r"), "ả") }
    func test_ax_composes_ã() { XCTAssertEqual(finalText("a", "x"), "ã") }
    func test_aj_composes_ạ() { XCTAssertEqual(finalText("a", "j"), "ạ") }

    func test_vietj_composes_việt() {
        XCTAssertEqual(finalText("v", "i", "e", "t", "j"), "việt")
    }

    func test_chuong_sequence_composes_chương() {
        XCTAssertEqual(finalText("c", "h", "u", "o", "w", "n", "g"), "chương")
    }

    func test_nguoiwf_composes_người() {
        XCTAssertEqual(finalText("n", "g", "u", "o", "w", "i", "f"), "người")
    }

    func test_tiengs_composes_tiếng() {
        XCTAssertEqual(finalText("t", "i", "e", "s", "n", "g"), "tiếng")
    }

    func test_backspace_from_việt_yields_việ() {
        let c = TelexComposer()
        for k in "vietj" { _ = c.onKey(k) }
        let r = c.onBackspace()
        if case .composing(let s) = r {
            XCTAssertEqual(s, "việ")
        } else {
            XCTFail("expected composing")
        }
    }

    func test_backspace_empty_composer_passes_through() {
        let r = TelexComposer().onBackspace()
        if case .passThrough = r {} else { XCTFail("expected passThrough") }
    }

    func test_backspace_from_â_yields_a() {
        let c = TelexComposer()
        for k in "aa" { _ = c.onKey(k) }
        let r = c.onBackspace()
        if case .composing(let s) = r {
            XCTAssertEqual(s, "a")
        } else {
            XCTFail("expected composing")
        }
    }

    func test_backspace_from_ậ_yields_â() {
        let c = TelexComposer()
        for k in "aaj" { _ = c.onKey(k) }
        let r = c.onBackspace()
        if case .composing(let s) = r {
            XCTAssertEqual(s, "â")
        } else {
            XCTFail("expected composing")
        }
    }

    func test_multiple_backspaces_fully_clear_viet_composing_state() {
        let c = TelexComposer()
        for k in "vietj" { _ = c.onKey(k) }
        var safety = 0
        while !c.currentComposingText().isEmpty && safety < 20 {
            _ = c.onBackspace()
            safety += 1
        }
        XCTAssertEqual(c.currentComposingText(), "")
        let again = c.onBackspace()
        if case .passThrough = again {} else { XCTFail("expected passThrough after fully cleared") }
    }

    func test_space_mid_cluster_commits_pending() {
        let c = TelexComposer()
        _ = c.onKey("v"); _ = c.onKey("i"); _ = c.onKey("e")
        let r = c.onKey(" ")
        if case .commit(let s) = r {
            XCTAssertEqual(s, "vie ")
        } else {
            XCTFail("expected commit")
        }
    }

    func test_reset_clears_state() {
        let c = TelexComposer()
        _ = c.onKey("a"); _ = c.onKey("a")
        c.reset()
        XCTAssertEqual(c.currentComposingText(), "")
    }

    // Tone cancel rules (Samsung-style retype cancels).
    func test_ass_cancels_tone_to_as() { XCTAssertEqual(finalText("a", "s", "s"), "as") }
    func test_aff_cancels_tone_to_af() { XCTAssertEqual(finalText("a", "f", "f"), "af") }
    func test_arr_cancels_tone_to_ar() { XCTAssertEqual(finalText("a", "r", "r"), "ar") }
    func test_axx_cancels_tone_to_ax() { XCTAssertEqual(finalText("a", "x", "x"), "ax") }
    func test_ajj_cancels_tone_to_aj() { XCTAssertEqual(finalText("a", "j", "j"), "aj") }
    func test_vietjj_keeps_circumflex_only() { XCTAssertEqual(finalText("v", "i", "e", "t", "j", "j"), "viêtj") }
    func test_aaa_cancels_circumflex() { XCTAssertEqual(finalText("a", "a", "a"), "aa") }
    func test_ooo_cancels_circumflex() { XCTAssertEqual(finalText("o", "o", "o"), "oo") }
    func test_eee_cancels_circumflex() { XCTAssertEqual(finalText("e", "e", "e"), "ee") }
    func test_aww_cancels_breve() { XCTAssertEqual(finalText("a", "w", "w"), "aw") }
    func test_oww_cancels_horn() { XCTAssertEqual(finalText("o", "w", "w"), "ow") }
    func test_uww_cancels_horn() { XCTAssertEqual(finalText("u", "w", "w"), "uw") }
    func test_uoww_cancels_uow_cluster() { XCTAssertEqual(finalText("u", "o", "w", "w"), "uow") }
    func test_ddd_cancels_d() { XCTAssertEqual(finalText("d", "d", "d"), "dd") }
    func test_ASS_preserves_case() { XCTAssertEqual(finalText("A", "S", "S"), "AS") }

    func test_length_cap_commits_at_32_chars() {
        let c = TelexComposer()
        var lastCommit: String?
        for _ in 0 ..< 33 {
            if case .commit(let s) = c.onKey("q") { lastCommit = s }
        }
        XCTAssertNotNil(lastCommit, "expected commit at length cap")
    }
}
