import XCTest
@testable import DraftRightKeyboardCore

final class KeyboardControllerTests: XCTestCase {

    private struct StubPack: LanguagePack {
        let id: String
        let displayName: String
        let locale = Locale(identifier: "en")
        let alphaRows: [[KeyDef]] = [[KeyDef("a", 97)]]
        let symbols1Rows: [[KeyDef]] = []
        let symbols2Rows: [[KeyDef]] = []
        let longPressAccents: [Character: [Character]] = [:]
        init(_ id: String) {
            self.id = id
            self.displayName = id.uppercased()
        }
    }

    func test_init_defaultsToFirstEnabledWhenActiveIdEmpty() {
        let reg = LanguageRegistry(packs: [EnglishLanguagePack()])
        let ctrl = KeyboardController(registry: reg, enabledIds: ["en"], activeId: "")
        XCTAssertEqual(ctrl.current.id, "en")
    }

    func test_init_honorsActiveIdWhenPresentInEnabled() {
        let reg = LanguageRegistry(packs: [StubPack("vi"), StubPack("fr"), StubPack("es")])
        let ctrl = KeyboardController(registry: reg, enabledIds: ["vi", "es"], activeId: "es")
        XCTAssertEqual(ctrl.current.id, "es")
    }

    func test_cycle_wrapsWithinEnabledSubset() {
        let reg = LanguageRegistry(packs: [StubPack("vi"), StubPack("fr"), StubPack("es")])
        let ctrl = KeyboardController(registry: reg, enabledIds: ["vi", "es"], activeId: "vi")
        XCTAssertEqual(ctrl.current.id, "vi")
        ctrl.cycleLanguage()
        XCTAssertEqual(ctrl.current.id, "es")
        ctrl.cycleLanguage()
        XCTAssertEqual(ctrl.current.id, "vi")
    }

    func test_cycle_reverseGoesBackwards() {
        let reg = LanguageRegistry(packs: [StubPack("en"), StubPack("vi"), StubPack("fr")])
        let ctrl = KeyboardController(registry: reg, enabledIds: ["en", "vi", "fr"], activeId: "en")
        ctrl.cycleLanguage(reverse: true)
        XCTAssertEqual(ctrl.current.id, "fr") // wraps backward
        ctrl.cycleLanguage(reverse: true)
        XCTAssertEqual(ctrl.current.id, "vi")
    }

    func test_cycle_isNoOpWhenSingleEnabled() {
        let reg = LanguageRegistry(packs: [EnglishLanguagePack()])
        let ctrl = KeyboardController(registry: reg, enabledIds: ["en"], activeId: "en")
        ctrl.cycleLanguage()
        XCTAssertEqual(ctrl.current.id, "en")
    }

    func test_disabledAllForceEnablesRegistryFirst() {
        let reg = LanguageRegistry(packs: [EnglishLanguagePack()])
        let ctrl = KeyboardController(registry: reg, enabledIds: [], activeId: "")
        XCTAssertEqual(ctrl.current.id, "en")
        XCTAssertEqual(ctrl.enabled.map { $0.id }, ["en"])
    }

    func test_setActiveSwitchesToEnabledPack() {
        let reg = LanguageRegistry(packs: [StubPack("vi"), StubPack("fr")])
        let ctrl = KeyboardController(registry: reg, enabledIds: ["vi", "fr"], activeId: "vi")
        ctrl.setActive(id: "fr")
        XCTAssertEqual(ctrl.current.id, "fr")
    }

    func test_setActiveNoOpForDisabledOrSameId() {
        let reg = LanguageRegistry(packs: [StubPack("vi"), StubPack("fr"), StubPack("es")])
        let ctrl = KeyboardController(registry: reg, enabledIds: ["vi", "fr"], activeId: "vi")
        ctrl.setActive(id: "es")
        XCTAssertEqual(ctrl.current.id, "vi")
        ctrl.setActive(id: "vi")
        XCTAssertEqual(ctrl.current.id, "vi")
    }

    func test_onKey_routesThroughTelexWhenVI() {
        let reg = LanguageRegistry.production
        let ctrl = KeyboardController(registry: reg, enabledIds: ["en", "vi"], activeId: "vi")
        _ = ctrl.onKey("v")
        _ = ctrl.onKey("i")
        _ = ctrl.onKey("e")
        _ = ctrl.onKey("t")
        let outcome = ctrl.onKey("j")
        if case .composing(let text) = outcome {
            XCTAssertEqual(text, "việt")
        } else {
            XCTFail("expected composing(việt) got \(outcome)")
        }
    }

    func test_onBackspaceNoChangeAfterEmptyingViBuffer() {
        let reg = LanguageRegistry.production
        let ctrl = KeyboardController(registry: reg, enabledIds: ["vi"], activeId: "vi")
        _ = ctrl.onKey("v")  // buffer = "v"
        let r = ctrl.onBackspace()  // buffer empties → .noChange
        XCTAssertEqual(r, .noChange)
    }

    func test_onKeyCommitForNonLetterAfterBuffer() {
        let reg = LanguageRegistry.production
        let ctrl = KeyboardController(registry: reg, enabledIds: ["vi"], activeId: "vi")
        _ = ctrl.onKey("v")
        _ = ctrl.onKey("i")
        _ = ctrl.onKey("e")
        let r = ctrl.onKey(" ")
        if case .commit(let text) = r {
            XCTAssertEqual(text, "vie ")
        } else {
            XCTFail("expected commit got \(r)")
        }
    }
}
