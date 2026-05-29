import Foundation

public struct FrenchLanguagePack: LanguagePack {
    public let id = "fr"
    public let displayName = "Français"
    public let locale = Locale(identifier: "fr")

    public init() {}

    public var alphaRows: [[KeyDef]] {
        [
            chars("a", "z", "e", "r", "t", "y", "u", "i", "o", "p"),
            chars("q", "s", "d", "f", "g", "h", "j", "k", "l", "m"),
            [KeyDef("⬆", SpecialKeys.shift, widthWeight: 1.5)]
                + chars("w", "x", "c", "v", "b", "n")
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

    public var symbols1Rows: [[KeyDef]] { EnglishLanguagePack().symbols1Rows }
    public var symbols2Rows: [[KeyDef]] { EnglishLanguagePack().symbols2Rows }

    public let longPressAccents: [Character: [Character]] = [
        "a": ["à", "â", "ä", "á", "ã"],
        "e": ["é", "è", "ê", "ë"],
        "i": ["î", "ï", "í", "ì"],
        "o": ["ô", "ö", "ó", "ò", "õ"],
        "u": ["ù", "û", "ü", "ú"],
        "c": ["ç"],
        "y": ["ÿ"],
    ]

    public static var appGroupContainer: URL?
    private static let wordlistPackPrefix = "draftright-wordlist-fr"

    public func makeCandidateEngine() -> CandidateEngine? {
        let wordList = WordListPackResolver.loadOrFallback(
            appGroupContainer: Self.appGroupContainer,
            packIdPrefix: Self.wordlistPackPrefix,
            fallback: { FrenchBootstrapWordList.wordList }
        )
        return TrigramCandidateEngine(wordList: wordList)
    }
}
