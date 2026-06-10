import Foundation

public struct VietnameseLanguagePack: LanguagePack {
    public let id = "vi"
    public let displayName = "Tiếng Việt"
    public let locale = Locale(identifier: "vi")
    public let longPressAccents: [Character: [Character]] = [:]

    private let base = EnglishLanguagePack()

    public init() {}

    public var alphaRows: [[KeyDef]] { base.alphaRows }
    public var symbols1Rows: [[KeyDef]] { base.symbols1Rows }
    public var symbols2Rows: [[KeyDef]] { base.symbols2Rows }

    /// App Group container URL — set by the keyboard view controller at
    /// startup so the resolver can look up an installed wordlist pack.
    /// Nil keeps the resolver in fallback-only mode (suitable for unit
    /// tests + the iOS Simulator without an App Group set up).
    public static var appGroupContainer: URL?

    /// Pack id prefix matching the backend manifest's wordlistPack URL.
    private static let wordlistPackPrefix = "draftright-wordlist-vi"

    public func makeComposer() -> Composer? { TelexComposer() }

    /// Trigram completions sourced from a downloaded wordlist pack when
    /// installed, otherwise the in-bundle bootstrap list. Engine is cheap
    /// to construct (shares the static `wordList`); the IME caller may
    /// still cache the result for the session.
    public func makeCandidateEngine() -> CandidateEngine? {
        let wordList = WordListPackResolver.loadOrFallback(
            appGroupContainer: Self.appGroupContainer,
            packIdPrefix: Self.wordlistPackPrefix,
            fallback: { VietnameseBootstrapWordList.wordList }
        )
        return TrigramCandidateEngine(wordList: wordList)
    }
}
