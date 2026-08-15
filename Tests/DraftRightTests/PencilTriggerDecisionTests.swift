import XCTest
@testable import DraftRight

/// The mouse-up → show-pencil decision (#177, refined #179/#180). The pencil
/// appears only when text is highlighted by dragging — never on a click,
/// including a double/triple click that selects a word or line.
final class PencilTriggerDecisionTests: XCTestCase {

    func testShowsOnDrag() {
        XCTAssertTrue(SelectionMonitor.shouldShowPencil(wasDragging: true))
    }

    func testHiddenWithoutDrag() {
        // A plain click or a double/triple click (no drag) must not show it,
        // even though a double-click selects a word that Accessibility reports.
        XCTAssertFalse(SelectionMonitor.shouldShowPencil(wasDragging: false))
    }
}
