import XCTest
@testable import DraftRight

/// The mouse-up → show-pencil decision (#177 → #181). The pencil appears only
/// when a drag actually selected text. A click never triggers it; a drag that
/// selected nothing (Accessibility confirms empty) doesn't either; but an
/// AX-blind app (selection unknown) still shows on a drag.
final class PencilTriggerDecisionTests: XCTestCase {

    func testShowsOnDragThatSelectedText() {
        // selectionKnownEmpty=false covers both "AX reports text" and
        // "AX can't read it" (Terminal) — either way, show on a drag.
        XCTAssertTrue(SelectionMonitor.shouldShowPencil(wasDragging: true, selectionKnownEmpty: false))
    }

    func testHiddenOnDragThatSelectedNothing() {
        // Drag over empty space: AX positively reports an empty selection.
        XCTAssertFalse(SelectionMonitor.shouldShowPencil(wasDragging: true, selectionKnownEmpty: true))
    }

    func testHiddenWithoutDrag() {
        // A plain/double/triple click (no drag) never shows it, whatever AX says.
        XCTAssertFalse(SelectionMonitor.shouldShowPencil(wasDragging: false, selectionKnownEmpty: false))
        XCTAssertFalse(SelectionMonitor.shouldShowPencil(wasDragging: false, selectionKnownEmpty: true))
    }
}
