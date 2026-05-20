import XCTest
@testable import DraftRightKeyboardCore

private final class SpyProxy: KeyboardTextProxy {
    var calls: [String] = []
    func insert(_ text: String) { calls.append("insert(\(text))") }
    func deleteBackward() { calls.append("deleteBackward") }
    func setComposing(_ text: String) { calls.append("compose(\(text))") }
    func clearComposing() { calls.append("clear") }
}

final class KeystrokeDispatcherTests: XCTestCase {

    private func dispatch(_ outcome: KeystrokeOutcome) -> [String] {
        let spy = SpyProxy()
        KeystrokeDispatcher.apply(outcome, to: spy)
        return spy.calls
    }

    func test_commit_clearsThenInserts() {
        XCTAssertEqual(dispatch(.commit("việt")), ["clear", "insert(việt)"])
    }

    func test_composing_setsMarkedText() {
        XCTAssertEqual(dispatch(.composing("vi")), ["compose(vi)"])
    }

    func test_deleteOne_deletesBackward() {
        XCTAssertEqual(dispatch(.deleteOne), ["deleteBackward"])
    }

    func test_noChange_clearsComposing() {
        // Regression: empty-buffer backspace must clear the marked region.
        XCTAssertEqual(dispatch(.noChange), ["clear"])
    }

    func test_commit_space_clearsMarkedThenInsertsSpace() {
        // Space routed through the composer commits the pending word and
        // appends the space — must not replace the marked region.
        XCTAssertEqual(dispatch(.commit("viet ")), ["clear", "insert(viet )"])
    }
}
