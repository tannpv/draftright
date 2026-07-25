import XCTest
@testable import DraftRightKeyboardCore

final class MicHoldGestureTests: XCTestCase {
    private let slop: CGFloat = 96

    func test_no_movement_is_not_cancel_armed() {
        XCTAssertFalse(MicHoldGesture.isCancelArmed(dx: 0, dy: 0, slop: slop))
    }

    func test_small_jitter_within_slop_is_not_cancel_armed() {
        XCTAssertFalse(MicHoldGesture.isCancelArmed(dx: 20, dy: -20, slop: slop))
    }

    func test_sliding_up_past_slop_is_cancel_armed() {
        XCTAssertTrue(MicHoldGesture.isCancelArmed(dx: 0, dy: -200, slop: slop))
    }

    func test_sliding_left_past_slop_is_cancel_armed() {
        XCTAssertTrue(MicHoldGesture.isCancelArmed(dx: -200, dy: 0, slop: slop))
    }

    func test_sliding_down_small_is_not_cancel_armed() {
        XCTAssertFalse(MicHoldGesture.isCancelArmed(dx: 0, dy: 40, slop: slop))
    }

    func test_sliding_down_far_is_not_cancel_armed() {
        XCTAssertFalse(MicHoldGesture.isCancelArmed(dx: 0, dy: 300, slop: slop))
    }

    func test_sliding_right_far_is_not_cancel_armed() {
        XCTAssertFalse(MicHoldGesture.isCancelArmed(dx: 300, dy: 0, slop: slop))
    }
}
