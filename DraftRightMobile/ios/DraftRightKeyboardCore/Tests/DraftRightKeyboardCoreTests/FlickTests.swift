import XCTest
@testable import DraftRightKeyboardCore

/// Japanese flick core (#212). Mirrors the Kotlin FlickTest 1:1.
final class FlickTests: XCTestCase {

    private let threshold = 30.0

    func testSmallMovementIsTap() {
        XCTAssertEqual(FlickGesture.resolve(dx: 0, dy: 0, tapThreshold: threshold), .tap)
        XCTAssertEqual(FlickGesture.resolve(dx: 10, dy: -12, tapThreshold: threshold), .tap)
    }

    func testDirectionsResolveByDominantAxis() {
        XCTAssertEqual(FlickGesture.resolve(dx: -100, dy: 5, tapThreshold: threshold), .left)
        XCTAssertEqual(FlickGesture.resolve(dx: 100, dy: -5, tapThreshold: threshold), .right)
        XCTAssertEqual(FlickGesture.resolve(dx: 5, dy: -100, tapThreshold: threshold), .up)
        XCTAssertEqual(FlickGesture.resolve(dx: -5, dy: 100, tapThreshold: threshold), .down)
    }

    func testUniformGojuonRows() {
        XCTAssertEqual(FlickLayout.kanaFor("あ", .left), "い")
        XCTAssertEqual(FlickLayout.kanaFor("か", .down), "こ")
        XCTAssertEqual(FlickLayout.kanaFor("た", .up), "つ")
        XCTAssertEqual(FlickLayout.kanaFor("さ", .right), "せ")
        XCTAssertEqual(FlickLayout.kanaFor("な", .tap), "な")
    }

    func testSpecialRows() {
        XCTAssertEqual(FlickLayout.kanaFor("や", .up), "ゆ")
        XCTAssertEqual(FlickLayout.kanaFor("や", .down), "よ")
        XCTAssertNil(FlickLayout.kanaFor("や", .left))
        XCTAssertEqual(FlickLayout.kanaFor("わ", .up), "ん")
        XCTAssertEqual(FlickLayout.kanaFor("わ", .left), "を")
        XCTAssertEqual(FlickLayout.kanaFor("わ", .right), "ー")
    }
}
