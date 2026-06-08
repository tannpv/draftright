import Foundation

public extension LanguageRegistry {
    /// Production registry containing all 7 Tier β language packs in
    /// cycle order. Matches Android LanguageRegistry.PRODUCTION.
    static let production: LanguageRegistry = LanguageRegistry(packs: [
        EnglishLanguagePack(),
        VietnameseLanguagePack(),
        FrenchLanguagePack(),
        SpanishLanguagePack(),
        GermanLanguagePack(),
        ItalianLanguagePack(),
        PortugueseLanguagePack(),
        JapaneseLanguagePack(),
        KoreanLanguagePack(),
    ])
}
