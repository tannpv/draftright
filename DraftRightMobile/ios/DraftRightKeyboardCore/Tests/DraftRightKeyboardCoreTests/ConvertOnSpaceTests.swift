import XCTest
@testable import DraftRightKeyboardCore

/// Which input methods convert the composing reading to the top candidate on
/// space (#207 Phase 2). Mirrors the Android `ConvertOnSpaceTest`: JP/ZH convert,
/// EN/VI/KO keep space literal.
final class ConvertOnSpaceTests: XCTestCase {

    func testReadingConversionPacksConvertOnSpace() {
        XCTAssertTrue(JapaneseLanguagePack().convertsOnSpace)
        XCTAssertTrue(ChineseLanguagePack().convertsOnSpace)
    }

    func testDirectAndPredictionPacksKeepSpaceLiteral() {
        XCTAssertFalse(EnglishLanguagePack().convertsOnSpace)
        XCTAssertFalse(VietnameseLanguagePack().convertsOnSpace)
        XCTAssertFalse(KoreanLanguagePack().convertsOnSpace)
    }
}
