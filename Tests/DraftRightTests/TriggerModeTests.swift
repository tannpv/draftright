import XCTest
@testable import DraftRight

/// The trigger-mode enum that lets pencil and hotkey coexist (#179).
final class TriggerModeTests: XCTestCase {

    func testPencilUsesPencilOnly() {
        XCTAssertTrue(TriggerMode.pencil.usesPencil)
        XCTAssertFalse(TriggerMode.pencil.usesHotkey)
    }

    func testHotkeyUsesHotkeyOnly() {
        XCTAssertFalse(TriggerMode.hotkey.usesPencil)
        XCTAssertTrue(TriggerMode.hotkey.usesHotkey)
    }

    func testBothUsesBoth() {
        XCTAssertTrue(TriggerMode.both.usesPencil)
        XCTAssertTrue(TriggerMode.both.usesHotkey)
    }

    func testRawValuesArePersistenceStable() {
        // These strings are written to UserDefaults — changing them silently
        // resets every user's saved mode. Pin them.
        XCTAssertEqual(TriggerMode.pencil.rawValue, "pencil")
        XCTAssertEqual(TriggerMode.hotkey.rawValue, "hotkey")
        XCTAssertEqual(TriggerMode.both.rawValue, "both")
        XCTAssertEqual(TriggerMode(rawValue: "both"), .both)
    }

    func testAllCasesCoverEveryMode() {
        XCTAssertEqual(Set(TriggerMode.allCases), [.pencil, .hotkey, .both])
    }
}
