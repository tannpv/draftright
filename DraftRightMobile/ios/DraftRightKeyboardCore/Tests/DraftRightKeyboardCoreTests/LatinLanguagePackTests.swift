import XCTest
@testable import DraftRightKeyboardCore

final class LatinLanguagePackTests: XCTestCase {

    // MARK: Vietnamese

    func test_vietnamese_idAndDisplayName() {
        let p = VietnameseLanguagePack()
        XCTAssertEqual(p.id, "vi")
        XCTAssertEqual(p.displayName, "Tiếng Việt")
    }

    func test_vietnamese_composerIsTelex() {
        let p = VietnameseLanguagePack()
        let a = p.makeComposer()
        let b = p.makeComposer()
        XCTAssertNotNil(a as? TelexComposer)
        XCTAssertNotNil(b as? TelexComposer)
        XCTAssertFalse((a as AnyObject) === (b as AnyObject), "factory must yield distinct instances")
    }

    func test_vietnamese_alphaRowsMirrorEnglish() {
        let vi = VietnameseLanguagePack()
        let en = EnglishLanguagePack()
        XCTAssertEqual(vi.alphaRows.count, en.alphaRows.count)
        XCTAssertEqual(vi.alphaRows[0].count, en.alphaRows[0].count)
    }

    // MARK: French

    func test_french_idAndDisplayName() {
        let p = FrenchLanguagePack()
        XCTAssertEqual(p.id, "fr")
        XCTAssertEqual(p.displayName, "Français")
    }

    func test_french_AZERTY_topRowStartsWithA() {
        XCTAssertEqual(FrenchLanguagePack().alphaRows[0].first?.label, "a")
    }

    func test_french_homeRowEndsWithM() {
        XCTAssertEqual(FrenchLanguagePack().alphaRows[1].last?.label, "m")
    }

    func test_french_accentsIncludeEAcuteGraveCirc() {
        let accents = FrenchLanguagePack().longPressAccents[Character("e")] ?? []
        for c in [Character("é"), Character("è"), Character("ê"), Character("ë")] {
            XCTAssertTrue(accents.contains(c), "missing \(c)")
        }
    }

    // MARK: Spanish

    func test_spanish_idAndDisplayName() {
        let p = SpanishLanguagePack()
        XCTAssertEqual(p.id, "es")
        XCTAssertEqual(p.displayName, "Español")
    }

    func test_spanish_homeRowEndsWithÑ() {
        XCTAssertEqual(SpanishLanguagePack().alphaRows[1].last?.label, "ñ")
    }

    func test_spanish_accentsIncludeInvertedPunctuation() {
        let p = SpanishLanguagePack()
        XCTAssertTrue((p.longPressAccents[Character("?")] ?? []).contains(Character("¿")))
        XCTAssertTrue((p.longPressAccents[Character("!")] ?? []).contains(Character("¡")))
    }

    // MARK: German

    func test_german_idAndDisplayName() {
        let p = GermanLanguagePack()
        XCTAssertEqual(p.id, "de")
        XCTAssertEqual(p.displayName, "Deutsch")
    }

    func test_german_QWERTZ_topRowSixthIsZ() {
        XCTAssertEqual(GermanLanguagePack().alphaRows[0][5].label, "z")
    }

    func test_german_alphaRowsIncludeUmlautsAndEszett() {
        let all = GermanLanguagePack().alphaRows.flatMap { $0 }.map { $0.label }
        for label in ["ä", "ö", "ü", "ß"] {
            XCTAssertTrue(all.contains(label), "missing \(label)")
        }
    }

    // MARK: Italian

    func test_italian_idAndDisplayName() {
        let p = ItalianLanguagePack()
        XCTAssertEqual(p.id, "it")
        XCTAssertEqual(p.displayName, "Italiano")
    }

    func test_italian_QWERTYIdenticalToEnglish() {
        let it = ItalianLanguagePack()
        let en = EnglishLanguagePack()
        XCTAssertEqual(it.alphaRows[0].map { $0.label }, en.alphaRows[0].map { $0.label })
        XCTAssertEqual(it.alphaRows[1].map { $0.label }, en.alphaRows[1].map { $0.label })
    }

    func test_italian_accentsLeadWithGraveForms() {
        let p = ItalianLanguagePack()
        XCTAssertEqual(p.longPressAccents[Character("a")]?.first, Character("à"))
        XCTAssertEqual(p.longPressAccents[Character("e")]?.first, Character("è"))
    }

    // MARK: Portuguese

    func test_portuguese_idAndDisplayName() {
        let p = PortugueseLanguagePack()
        XCTAssertEqual(p.id, "pt")
        XCTAssertEqual(p.displayName, "Português")
    }

    func test_portuguese_homeRowEndsWithÇ() {
        XCTAssertEqual(PortugueseLanguagePack().alphaRows[1].last?.label, "ç")
    }

    func test_portuguese_accentsIncludeTildeForms() {
        let p = PortugueseLanguagePack()
        XCTAssertTrue((p.longPressAccents[Character("a")] ?? []).contains(Character("ã")))
        XCTAssertTrue((p.longPressAccents[Character("o")] ?? []).contains(Character("õ")))
    }

    // MARK: Production registry

    func test_productionRegistryHasAllPacksInOrder() {
        let reg = LanguageRegistry.production
        let ids = reg.all.map { $0.id }
        XCTAssertEqual(ids, ["en", "vi", "fr", "es", "de", "it", "pt", "ja", "ko"])
    }
}
