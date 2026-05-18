import Foundation

public struct ItalianLanguagePack: LanguagePack {
    public let id = "it"
    public let displayName = "Italiano"
    public let locale = Locale(identifier: "it")

    private let base = EnglishLanguagePack()

    public init() {}

    public var alphaRows: [[KeyDef]] { base.alphaRows }
    public var symbols1Rows: [[KeyDef]] { base.symbols1Rows }
    public var symbols2Rows: [[KeyDef]] { base.symbols2Rows }

    public let longPressAccents: [Character: [Character]] = [
        "a": ["à", "á", "â", "ã", "ä"],
        "e": ["è", "é", "ê", "ë"],
        "i": ["ì", "í", "î", "ï"],
        "o": ["ò", "ó", "ô", "õ", "ö"],
        "u": ["ù", "ú", "û", "ü"],
    ]
}
