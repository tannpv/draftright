import XCTest
@testable import DraftRightKeyboardCore

/// One-shot undo armed by an auto-correction (#207) — mirror of Android
/// `AutoCorrectUndoTest`. The backspace right after a correction puts the typed
/// word back instead of deleting a character.
final class AutoCorrectUndoTests: XCTestCase {

    func testBackspaceAfterCorrectionRevertsOnce() {
        let undo = AutoCorrectUndo()
        undo.arm(original: "khôg", corrected: "không")
        XCTAssertEqual(undo.corrected, "không")
        XCTAssertEqual(undo.consume(), "khôg")
        XCTAssertNil(undo.consume(), "undo is one-shot")
    }

    func testNothingToConsumeBeforeAnyCorrection() {
        XCTAssertNil(AutoCorrectUndo().consume())
    }

    func testDisarmDropsThePendingUndo() {
        let undo = AutoCorrectUndo()
        undo.arm(original: "khôg", corrected: "không")
        undo.disarm()
        XCTAssertNil(undo.consume())
    }

    func testArmingAgainReplacesThePendingUndo() {
        let undo = AutoCorrectUndo()
        undo.arm(original: "khôg", corrected: "không")
        undo.arm(original: "anb", corrected: "anh")
        XCTAssertEqual(undo.consume(), "anb")
    }

    func testIsArmedTracksTheOneShotState() {
        let undo = AutoCorrectUndo()
        XCTAssertFalse(undo.isArmed)
        undo.arm(original: "khôg", corrected: "không")
        XCTAssertTrue(undo.isArmed)
        _ = undo.consume()
        XCTAssertFalse(undo.isArmed)
    }
}
