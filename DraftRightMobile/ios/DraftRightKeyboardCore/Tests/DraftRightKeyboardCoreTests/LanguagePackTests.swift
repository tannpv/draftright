import XCTest
@testable import DraftRightKeyboardCore

final class LanguagePackTests: XCTestCase {

    private struct StubPack: LanguagePack {
        let id = "stub"
        let displayName = "Stub"
        let locale = Locale(identifier: "en")
        let alphaRows: [[KeyDef]] = [[KeyDef("a", 97), KeyDef("b", 98)]]
        let symbols1Rows: [[KeyDef]] = []
        let symbols2Rows: [[KeyDef]] = []
        let longPressAccents: [Character: [Character]] = [:]
    }

    func test_idAndDisplayNameAreExposed() {
        let p = StubPack()
        XCTAssertEqual(p.id, "stub")
        XCTAssertEqual(p.displayName, "Stub")
    }

    func test_composerFactoryDefaultsToNil() {
        XCTAssertNil(StubPack().makeComposer())
    }

    func test_keyDefCarriesLabelCodeAndWidthWeight() {
        let k = KeyDef("ñ", 241, widthWeight: 1.5)
        XCTAssertEqual(k.label, "ñ")
        XCTAssertEqual(k.code, 241)
        XCTAssertEqual(k.widthWeight, 1.5)
    }
}
