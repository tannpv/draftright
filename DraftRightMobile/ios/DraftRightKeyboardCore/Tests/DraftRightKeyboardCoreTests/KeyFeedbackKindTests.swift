import XCTest
@testable import DraftRightKeyboardCore

/// Locks the per-kind haptic mapping (#209): destructive/commit keys feel firmer
/// than typing keys. The pure half of iOS key feedback — the click + haptic
/// firing touches UIKit and is verified on-device. TC: KBD-FEEDBACK-IOS-1
final class KeyFeedbackKindTests: XCTestCase {

    func testTypingKeysAreLight() {
        XCTAssertEqual(KeyFeedbackKind.char.impact, .light)
        XCTAssertEqual(KeyFeedbackKind.space.impact, .light)
        XCTAssertEqual(KeyFeedbackKind.other.impact, .light)
    }

    func testDestructiveAndCommitKeysAreMedium() {
        XCTAssertEqual(KeyFeedbackKind.delete.impact, .medium)
        XCTAssertEqual(KeyFeedbackKind.enter.impact, .medium)
    }
}
