import XCTest
@testable import DraftRight

/// The trigger-mode enum — pencil or hotkey, mutually exclusive (#179, #180).
final class TriggerModeTests: XCTestCase {

    func testPencilUsesPencilOnly() {
        XCTAssertTrue(TriggerMode.pencil.usesPencil)
        XCTAssertFalse(TriggerMode.pencil.usesHotkey)
    }

    func testHotkeyUsesHotkeyOnly() {
        XCTAssertFalse(TriggerMode.hotkey.usesPencil)
        XCTAssertTrue(TriggerMode.hotkey.usesHotkey)
    }

    func testModesAreMutuallyExclusive() {
        for mode in TriggerMode.allCases {
            XCTAssertNotEqual(mode.usesPencil, mode.usesHotkey, "exactly one mechanism per mode")
        }
    }

    func testRawValuesArePersistenceStable() {
        // These strings are written to UserDefaults — changing them silently
        // resets every user's saved mode. Pin them.
        XCTAssertEqual(TriggerMode.pencil.rawValue, "pencil")
        XCTAssertEqual(TriggerMode.hotkey.rawValue, "hotkey")
        XCTAssertEqual(TriggerMode(rawValue: "hotkey"), .hotkey)
        // A removed mode ("both") no longer decodes — the caller migrates it.
        XCTAssertNil(TriggerMode(rawValue: "both"))
    }

    func testAllCasesCoverEveryMode() {
        XCTAssertEqual(Set(TriggerMode.allCases), [.pencil, .hotkey])
    }
}
