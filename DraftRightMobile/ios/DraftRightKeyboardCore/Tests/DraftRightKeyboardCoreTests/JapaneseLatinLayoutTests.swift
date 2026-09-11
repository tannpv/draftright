import XCTest
@testable import DraftRightKeyboardCore

/// Japanese types on the LATIN keyboard: the ja pack renders the same ASCII
/// QWERTY as English and converts rōmaji to kana in the composer, never in the
/// layout. The 12-key kana grid is opt-in (jpFlick, #212).
///
/// This is the machine check behind "the Japanese keyboard IS the latin one" —
/// a kana key sneaking into the ja rows, or the flick composer being picked
/// while the flick switch is off, would silently change what users see.
/// Mirror of Kotlin JapaneseLatinLayoutTest.
final class JapaneseLatinLayoutTests: XCTestCase {

    private let japanese = JapaneseLanguagePack()
    private let english = EnglishLanguagePack()

    private func controller(activeId: String, jpFlick: Bool) -> KeyboardController {
        KeyboardController(
            registry: LanguageRegistry(packs: [EnglishLanguagePack(), JapaneseLanguagePack()]),
            enabledIds: ["en", "ja"],
            activeId: activeId,
            jpFlick: jpFlick
        )
    }

    func testJapaneseRendersTheSameRowsAsEnglish() {
        XCTAssertEqual(english.alphaRows, japanese.alphaRows)
        XCTAssertEqual(english.symbols1Rows, japanese.symbols1Rows)
        XCTAssertEqual(english.symbols2Rows, japanese.symbols2Rows)
    }

    func testJapaneseAlphaKeysAreLatinLetters() {
        let letters = japanese.alphaRows.flatMap { $0 }
            .map { $0.label }
            .filter { $0.count == 1 && $0.first!.isLetter }
        XCTAssertEqual(letters.count, 26, "a QWERTY alpha layer has 26 letter keys")
        for label in letters {
            XCTAssertTrue(
                label.first!.isASCII && label.first!.isLowercase,
                "non-ASCII key '\(label)' in the Japanese alpha rows"
            )
        }
    }

    func testRomajiComposerIsUsedWhenFlickIsOff() {
        XCTAssertTrue(
            controller(activeId: "ja", jpFlick: false).composer is RomajiKanaComposer,
            "flick off must keep the rōmaji composer"
        )
    }

    func testFlickComposerIsUsedOnlyWhenTheSwitchIsOn() {
        XCTAssertTrue(
            controller(activeId: "ja", jpFlick: true).composer is KanaComposer,
            "flick on must swap in the kana composer"
        )
    }

    func testFlickSwitchNeverAffectsOtherLanguages() {
        XCTAssertFalse(
            controller(activeId: "en", jpFlick: true).composer is KanaComposer,
            "English must never get the kana composer"
        )
    }

    func testTypingRomajiOnTheLatinKeysProducesKana() {
        let composer = japanese.makeComposer()
        for key in "konnichiha" { _ = composer?.onKey(key) }
        XCTAssertEqual(composer?.currentComposingText(), "こんにちは")
    }
}
