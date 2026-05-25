import XCTest
@testable import DraftRightKeyboardCore

final class LanguageRegistryTests: XCTestCase {

    private struct StubPack: LanguagePack {
        let id: String
        let displayName: String
        let locale = Locale(identifier: "en")
        let alphaRows: [[KeyDef]] = []
        let symbols1Rows: [[KeyDef]] = []
        let symbols2Rows: [[KeyDef]] = []
        let longPressAccents: [Character: [Character]] = [:]

        init(_ id: String) {
            self.id = id
            self.displayName = id.uppercased()
        }
    }

    func test_byId_returnsTheMatchingPack() {
        let reg = LanguageRegistry(packs: [StubPack("en"), StubPack("vi")])
        XCTAssertEqual(reg.byId("vi").id, "vi")
    }

    func test_byIdOrDefault_fallsBackToFirstWhenIdUnknown() {
        let reg = LanguageRegistry(packs: [StubPack("en"), StubPack("vi")])
        XCTAssertEqual(reg.byIdOrDefault("zz").id, "en")
    }

    func test_next_cyclesInOrderWithWrapAround() {
        let reg = LanguageRegistry(packs: [StubPack("vi"), StubPack("fr"), StubPack("es")])
        XCTAssertEqual(reg.next(currentId: "vi").id, "fr")
        XCTAssertEqual(reg.next(currentId: "fr").id, "es")
        XCTAssertEqual(reg.next(currentId: "es").id, "vi")
    }

    func test_next_onUnknownIdReturnsFirst() {
        let reg = LanguageRegistry(packs: [StubPack("en"), StubPack("vi")])
        XCTAssertEqual(reg.next(currentId: "xx").id, "en")
    }
}
