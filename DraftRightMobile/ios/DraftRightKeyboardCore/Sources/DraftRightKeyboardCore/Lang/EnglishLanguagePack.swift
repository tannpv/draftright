import Foundation

public struct EnglishLanguagePack: LanguagePack {
    public let id = "en"
    public let displayName = "English"
    public let locale = Locale(identifier: "en")
    public let longPressAccents: [Character: [Character]] = [:]

    /// App Group container — set by KeyboardViewController. Nil keeps the
    /// resolver in fallback-only mode (sim + unit tests).
    public static var appGroupContainer: URL?

    /// Pack id prefix matching the backend manifest's wordlistPack URL.
    private static let wordlistPackPrefix = "draftright-wordlist-en"

    public init() {}

    /// Trigram completions sourced from a downloaded wordlist pack when
    /// installed, otherwise the in-bundle bootstrap list. Same shape as
    /// VietnameseLanguagePack — Rule #1 (reusable pattern).
    public func makeCandidateEngine() -> CandidateEngine? {
        let wordList = WordListPackResolver.loadOrFallback(
            appGroupContainer: Self.appGroupContainer,
            packIdPrefix: Self.wordlistPackPrefix,
            fallback: { EnglishBootstrapWordList.wordList }
        )
        return TrigramCandidateEngine(wordList: wordList)
    }

    // Standard ASCII QWERTY, shared with the Japanese rōmaji pack (Rule #1).
    public var alphaRows: [[KeyDef]] { QwertyLayout.alphaRows }
    public var symbols1Rows: [[KeyDef]] { QwertyLayout.symbols1Rows }
    public var symbols2Rows: [[KeyDef]] { QwertyLayout.symbols2Rows }
}
