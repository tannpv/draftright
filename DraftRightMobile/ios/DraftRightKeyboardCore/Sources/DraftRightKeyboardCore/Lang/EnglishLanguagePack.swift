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

    public var alphaRows: [[KeyDef]] {
        [
            chars("q", "w", "e", "r", "t", "y", "u", "i", "o", "p"),
            chars("a", "s", "d", "f", "g", "h", "j", "k", "l"),
            [KeyDef("⬆", SpecialKeys.shift, widthWeight: 1.5)]
                + chars("z", "x", "c", "v", "b", "n", "m")
                + [KeyDef("←", SpecialKeys.backspace, widthWeight: 1.5)],
            [
                KeyDef("?123", SpecialKeys.symbols, widthWeight: 1.5),
                KeyDef("🌐", SpecialKeys.globe, widthWeight: 1.0),
                KeyDef(",", keyCode(","), widthWeight: 1.0),
                KeyDef(" ", keyCode(" "), widthWeight: 4.0),
                KeyDef(".", keyCode("."), widthWeight: 1.0),
                KeyDef("↵", SpecialKeys.enter, widthWeight: 1.5),
            ],
        ]
    }

    public var symbols1Rows: [[KeyDef]] {
        [
            chars("1", "2", "3", "4", "5", "6", "7", "8", "9", "0"),
            chars("@", "#", "$", "%", "&", "-", "+", "(", ")"),
            [KeyDef("#+=", SpecialKeys.symbols2, widthWeight: 1.5)]
                + chars("!", "\"", "'", ":", ";", "/", "?")
                + [KeyDef("←", SpecialKeys.backspace, widthWeight: 1.5)],
            [
                KeyDef("ABC", SpecialKeys.alpha, widthWeight: 1.5),
                KeyDef("🌐", SpecialKeys.globe, widthWeight: 1.0),
                KeyDef(",", keyCode(","), widthWeight: 1.0),
                KeyDef(" ", keyCode(" "), widthWeight: 4.0),
                KeyDef(".", keyCode("."), widthWeight: 1.0),
                KeyDef("↵", SpecialKeys.enter, widthWeight: 1.5),
            ],
        ]
    }

    public var symbols2Rows: [[KeyDef]] {
        [
            chars("~", "`", "|", "•", "√", "π", "÷", "×", "¶", "Δ"),
            chars("£", "€", "¥", "^", "[", "]", "{", "}"),
            [KeyDef("?123", SpecialKeys.symbols, widthWeight: 1.5)]
                + chars("©", "®", "™", "\\", "<", ">", "=")
                + [KeyDef("←", SpecialKeys.backspace, widthWeight: 1.5)],
            [
                KeyDef("ABC", SpecialKeys.alpha, widthWeight: 1.5),
                KeyDef("🌐", SpecialKeys.globe, widthWeight: 1.0),
                KeyDef(",", keyCode(","), widthWeight: 1.0),
                KeyDef(" ", keyCode(" "), widthWeight: 4.0),
                KeyDef(".", keyCode("."), widthWeight: 1.0),
                KeyDef("↵", SpecialKeys.enter, widthWeight: 1.5),
            ],
        ]
    }
}
