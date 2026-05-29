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

    public func makeComposer() -> Composer? { TelexComposer() }

    /// Trigram completions sourced from the bootstrap word list. Engine
    /// is cheap to construct (shares the static `wordList`); the IME
    /// caller may still cache the result for the session.
    public func makeCandidateEngine() -> CandidateEngine? {
        TrigramCandidateEngine(wordList: VietnameseBootstrapWordList.wordList)
    }
}
