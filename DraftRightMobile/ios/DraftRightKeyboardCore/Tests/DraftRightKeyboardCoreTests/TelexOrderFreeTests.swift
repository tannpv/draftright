import XCTest
@testable import DraftRightKeyboardCore

/// Samsung-parity order-free marking (spec 2026-09-11): tone keys, w, doubled
/// vowels, and trailing d commute at word scope. Every expected value below was
/// verified against Samsung Honeyboard VI-Telex on a Galaxy A52 (2026-09-11) or
/// derived from the same rule. Mirror of Kotlin TelexOrderFreeTest.
final class TelexOrderFreeTests: XCTestCase {

    private func type(_ keys: String) -> String {
        let c = TelexComposer()
        var last: ComposeResult = .passThrough
        for k in keys { last = c.onKey(k) }
        switch last {
        case .composing(let s), .commit(let s): return s
        default: return c.currentComposingText()
        }
    }

    // --- Mechanic 1: quality modifier AFTER tone (tone-transparent) ---
    func testToneThenHorn() {
        XCTAssertEqual(type("tuongrw"), "tưởng")   // device-verified vs Samsung
        XCTAssertEqual(type("nguoifw"), "người")   // device-verified vs Samsung
        XCTAssertEqual(type("muasw"), "mứa")       // tone must MOVE u→ư target
    }

    func testToneThenCircumflex() {
        XCTAssertEqual(type("cansja"), "cận")      // tone j then late aa
        XCTAssertEqual(type("ddongfo"), "đồng")    // tone f then late oo
    }

    // --- Mechanic 2: quality marks REPLACE each other (root matching) ---
    func testHornOverridesCircumflex() {
        XCTAssertEqual(type("oow"), "ơ")            // ô + w → ơ
        XCTAssertEqual(type("duocjdw"), "được")     // j promotes uo→uô, w re-horns
    }

    func testCircumflexOverridesHorn() {
        XCTAssertEqual(type("owo"), "ô")            // ơ + o → ô
    }

    // --- Mechanic 3: remote đ ---
    func testRemoteD() {
        XCTAssertEqual(type("duocd"), "đuoc")       // trailing d pairs with initial
        XCTAssertEqual(type("duocdwj"), "được")     // device-verified vs Samsung
        XCTAssertEqual(type("Duocdwj"), "Được")     // case preserved from initial
        XCTAssertEqual(type("dungfd"), "đùng")      // remote d after tone
    }

    func testRemoteDCancel() {
        XCTAssertEqual(type("duocdd"), "duocd")     // second remote d reverts + literal
    }

    func testRemoteDDoesNotFireWithoutDInitial() {
        XCTAssertEqual(type("bad"), "bad")          // d stays literal
    }

    // --- Full permutation of the study word: every mark order → Được ---
    func testDuocAllMarkOrders() {
        for marks in ["dwj", "djw", "wdj", "wjd", "jdw", "jwd"] {
            XCTAssertEqual(type("duoc\(marks)"), "được", "marks=\(marks)")
        }
    }

    // --- Must-not-regress spot checks (already green today; guard rail) ---
    func testExistingBehavioursHold() {
        XCTAssertEqual(type("tuongwr"), "tưởng")
        XCTAssertEqual(type("khongo"), "không")
        XCTAssertEqual(type("hoaf"), "hòa")
        XCTAssertEqual(type("nguyeenx"), "nguyễn")
        XCTAssertEqual(type("ruwowuj"), "rượu")
    }

    // --- liftTone: tone removal for order-free marking ---
    func testLiftToneStripsAndReports() {
        var (bare, tone) = TelexComposer.liftTone("tuỏng")
        XCTAssertEqual(bare, "tuong")
        XCTAssertEqual(tone, "r")

        (bare, tone) = TelexComposer.liftTone("tưởng")  // quality marks stay
        XCTAssertEqual(bare, "tương")
        XCTAssertEqual(tone, "r")

        (bare, tone) = TelexComposer.liftTone("duoc")
        XCTAssertEqual(bare, "duoc")
        XCTAssertNil(tone)

        (bare, tone) = TelexComposer.liftTone("Hòa")    // case preserved
        XCTAssertEqual(bare, "Hoa")
        XCTAssertEqual(tone, "f")
    }
}
