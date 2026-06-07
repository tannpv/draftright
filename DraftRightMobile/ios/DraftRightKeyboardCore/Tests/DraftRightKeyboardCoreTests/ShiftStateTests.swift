import XCTest
@testable import DraftRightKeyboardCore

/// Locks the single-tap shift cycle to match the stock Samsung keyboard:
/// each tap advances off -> single (cap first) -> capsLock (cap all) -> off.
/// No double-tap; one button cycles all three states.
final class ShiftStateTests: XCTestCase {

    func testTapCycleAdvancesThroughAllThreeStates() {
        XCTAssertEqual(ShiftState.off.nextOnTap(), .single)
        XCTAssertEqual(ShiftState.single.nextOnTap(), .capsLock)
        XCTAssertEqual(ShiftState.capsLock.nextOnTap(), .off)
    }

    func testThreeTapsReturnToStart() {
        XCTAssertEqual(ShiftState.off.nextOnTap().nextOnTap().nextOnTap(), .off)
    }

    func testEachStateHasADistinctIconName() {
        let names = [ShiftState.off, .single, .capsLock].map { $0.iconName }
        XCTAssertEqual(Set(names).count, names.count)
        names.forEach { XCTAssertFalse($0.isEmpty) }
    }
}
