import XCTest
@testable import DraftRightKeyboardCore

final class HangulAssemblerTests: XCTestCase {
    private func a(_ s: String) -> String { HangulAssembler.assemble(s) }

    func testBlock() { XCTAssertEqual(a("ㄱㅏ"), "가") }
    func testCVC() { XCTAssertEqual(a("ㅎㅏㄴ"), "한") }
    func testAnnyeong() { XCTAssertEqual(a("ㅇㅏㄴㄴㅕㅇ"), "안녕") }
    func testFinalMoves() { XCTAssertEqual(a("ㄱㅏㄴㅣ"), "가니") }
    func testCompoundVowel() { XCTAssertEqual(a("ㄱㅗㅏ"), "과") }
    func testCompoundFinal() { XCTAssertEqual(a("ㄱㅏㅂㅅ"), "값") }
    func testCompoundFinalSplits() { XCTAssertEqual(a("ㄱㅏㅂㅅㅓ"), "갑서") }
    func testLoneConsonant() { XCTAssertEqual(a("ㄱ"), "ㄱ") }
    func testEmpty() { XCTAssertEqual(a(""), "") }
}
