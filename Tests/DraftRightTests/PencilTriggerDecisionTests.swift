import XCTest
@testable import DraftRight

/// The mouse-up → show-pencil decision (#177, refined #179). AX-friendly apps
/// report selected text; AX-blind apps (Terminal) don't, so a drag carries the
/// decision. A double/triple click is deliberately not a trigger.
final class PencilTriggerDecisionTests: XCTestCase {

    func testShowsWhenAccessibilityReportsSelection() {
        // AX text present (e.g. keyboard select, or a word-select in Notes).
        XCTAssertTrue(SelectionMonitor.shouldShowPencil(hasAXText: true, wasDragging: false))
    }

    func testShowsOnDragWhenNoAXText() {
        // Terminal: no AX text, but the user drag-highlighted.
        XCTAssertTrue(SelectionMonitor.shouldShowPencil(hasAXText: false, wasDragging: true))
    }

    func testHiddenOnClickSelectWithoutDragOrAXText() {
        // A double/triple click in an AX-blind app is NOT a trigger — it fires
        // while merely reading. Only a drag (or AX text) shows the pencil (#179).
        XCTAssertFalse(SelectionMonitor.shouldShowPencil(hasAXText: false, wasDragging: false))
    }
}
