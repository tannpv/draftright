import XCTest
@testable import DraftRight

/// The mouse-up → show-pencil decision (#177). AX-friendly apps report selected
/// text; AX-blind apps (Terminal) don't, so the gesture must carry the decision.
/// This guards the regression where the pencil never appeared in Terminal.
final class PencilTriggerDecisionTests: XCTestCase {

    func testShowsWhenAccessibilityReportsSelection() {
        // AX text present → show regardless of gesture.
        XCTAssertTrue(SelectionMonitor.shouldShowPencil(hasAXText: true, wasDragging: false, clickCount: 1))
    }

    func testShowsOnDragWhenNoAXText() {
        // Terminal: no AX text, but the user drag-selected.
        XCTAssertTrue(SelectionMonitor.shouldShowPencil(hasAXText: false, wasDragging: true, clickCount: 1))
    }

    func testShowsOnDoubleClickWordSelectWhenNoAXText() {
        XCTAssertTrue(SelectionMonitor.shouldShowPencil(hasAXText: false, wasDragging: false, clickCount: 2))
    }

    func testShowsOnTripleClickLineSelectWhenNoAXText() {
        XCTAssertTrue(SelectionMonitor.shouldShowPencil(hasAXText: false, wasDragging: false, clickCount: 3))
    }

    func testHiddenOnPlainSingleClickWithNoSelection() {
        // A bare click that selected nothing must NOT surface the pencil.
        XCTAssertFalse(SelectionMonitor.shouldShowPencil(hasAXText: false, wasDragging: false, clickCount: 1))
    }

    func testHiddenWhenNothingIndicatesSelection() {
        XCTAssertFalse(SelectionMonitor.shouldShowPencil(hasAXText: false, wasDragging: false, clickCount: 0))
    }
}
