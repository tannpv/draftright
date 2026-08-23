import XCTest
@testable import DraftRightKeyboardCore

/// #190 — the digits-layer decision for a field's UIKeyboardType raw value.
final class NumericFieldTests: XCTestCase {

    // UIKeyboardType raw values, restated so the test asserts against the real
    // UIKit ABI (not the copy inside NumericField).
    private let defaultType = 0
    private let asciiCapable = 1
    private let numbersAndPunctuation = 2
    private let url = 3
    private let numberPad = 4
    private let phonePad = 5
    private let namePhonePad = 6
    private let emailAddress = 7
    private let decimalPad = 8
    private let asciiCapableNumberPad = 11

    func testDigitOnlyPadsUseDigitsLayer() {
        XCTAssertTrue(NumericField.isNumericKeyboard(numberPad))
        XCTAssertTrue(NumericField.isNumericKeyboard(phonePad))
        XCTAssertTrue(NumericField.isNumericKeyboard(decimalPad))
        XCTAssertTrue(NumericField.isNumericKeyboard(asciiCapableNumberPad))
    }

    func testTextAndMixedTypesUseAlpha() {
        for kind in [defaultType, asciiCapable, url, emailAddress, namePhonePad] {
            XCTAssertFalse(NumericField.isNumericKeyboard(kind))
        }
        // numbersAndPunctuation is a full keyboard (numbers row + letters via
        // shift), not a digits-only pad — stays on alpha, matching stock.
        XCTAssertFalse(NumericField.isNumericKeyboard(numbersAndPunctuation))
    }
}
