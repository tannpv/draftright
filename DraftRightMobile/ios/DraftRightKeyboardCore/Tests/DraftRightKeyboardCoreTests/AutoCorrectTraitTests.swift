import XCTest
@testable import DraftRightKeyboardCore

/// Auto-correct-on-space (#207) is a pack trait, never a check on the language
/// name — mirror of Android `AutoCorrectTraitTest`. Vietnamese ships the ~8.5k
/// frequency list, so it's the only pack opted in today.
final class AutoCorrectTraitTests: XCTestCase {

    func testVietnameseOptsIn() {
        XCTAssertTrue(VietnameseLanguagePack().autoCorrectEnabled)
    }

    func testOtherPacksAreOffByDefault() {
        XCTAssertFalse(EnglishLanguagePack().autoCorrectEnabled)
        XCTAssertFalse(JapaneseLanguagePack().autoCorrectEnabled)
    }
}
