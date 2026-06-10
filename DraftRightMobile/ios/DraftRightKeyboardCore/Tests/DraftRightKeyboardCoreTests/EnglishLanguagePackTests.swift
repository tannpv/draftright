import XCTest
@testable import DraftRightKeyboardCore

final class EnglishLanguagePackTests: XCTestCase {
    func test_idAndDisplayName() {
        let p = EnglishLanguagePack()
        XCTAssertEqual(p.id, "en")
        XCTAssertEqual(p.displayName, "English")
    }

    func test_alphaRowsHasFourRows() {
        XCTAssertEqual(EnglishLanguagePack().alphaRows.count, 4)
    }

    func test_topRowStartsWithQEndsWithP() {
        let top = EnglishLanguagePack().alphaRows[0]
        XCTAssertEqual(top.first?.label, "q")
        XCTAssertEqual(top.last?.label, "p")
        XCTAssertEqual(top.count, 10)
    }

    func test_homeRowStartsWithAEndsWithL() {
        let home = EnglishLanguagePack().alphaRows[1]
        XCTAssertEqual(home.first?.label, "a")
        XCTAssertEqual(home.last?.label, "l")
        XCTAssertEqual(home.count, 9)
    }

    func test_composerFactoryReturnsPassthrough() {
        XCTAssertTrue(EnglishLanguagePack().makeComposer() is PassthroughComposer)
    }

    func test_longPressAccentsIsEmpty() {
        XCTAssertTrue(EnglishLanguagePack().longPressAccents.isEmpty)
    }

    func test_symbols1FirstRowIsDigits1Through0() {
        let digits = EnglishLanguagePack().symbols1Rows[0]
        XCTAssertEqual(digits.first?.label, "1")
        XCTAssertEqual(digits.last?.label, "0")
        XCTAssertEqual(digits.count, 10)
    }

    func test_symbols2FirstRowContainsTildeAndPi() {
        let row = EnglishLanguagePack().symbols2Rows[0]
        XCTAssertEqual(row.first?.label, "~")
        XCTAssertTrue(row.contains(where: { $0.label == "π" }))
    }

    func test_symbols1SecondRowContainsUnderscore() {
        let row = EnglishLanguagePack().symbols1Rows[1]
        XCTAssertTrue(row.contains(where: { $0.label == "_" }))
    }
}
