import XCTest
@testable import DraftRightKeyboardCore

final class TelexStateTests: XCTestCase {
    func test_isVowel_detectsPlainVowelsAndUppercase() {
        for c in ["a", "e", "i", "o", "u", "y", "A", "E", "I", "O", "U", "Y"] {
            XCTAssertTrue(TelexState.isVowel(Character(c)), "\(c) should be vowel")
        }
        for c in ["b", "q", "z"] {
            XCTAssertFalse(TelexState.isVowel(Character(c)), "\(c) should not be vowel")
        }
    }

    func test_isVowelLike_includesSpecialVnVowels() {
        for c in ["ă", "â", "ê", "ô", "ơ", "ư"] {
            XCTAssertTrue(TelexState.isVowelLike(Character(c)), "\(c) should be vowel-like")
        }
    }

    func test_isToneMark_detectsSFRXJ() {
        for c in ["s", "f", "r", "x", "j"] {
            XCTAssertTrue(TelexState.isToneMark(Character(c)), "\(c) should be tone mark")
        }
        XCTAssertFalse(TelexState.isToneMark("a"))
    }
}
