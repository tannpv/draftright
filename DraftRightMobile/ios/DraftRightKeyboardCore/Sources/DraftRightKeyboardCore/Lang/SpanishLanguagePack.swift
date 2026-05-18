import Foundation

private func chars(_ labels: String...) -> [KeyDef] {
    labels.map { KeyDef($0, Int($0.unicodeScalars.first!.value)) }
}

public struct SpanishLanguagePack: LanguagePack {
    public let id = "es"
    public let displayName = "Español"
    public let locale = Locale(identifier: "es")

    public init() {}

    public var alphaRows: [[KeyDef]] {
        [
            chars("q", "w", "e", "r", "t", "y", "u", "i", "o", "p"),
            chars("a", "s", "d", "f", "g", "h", "j", "k", "l", "ñ"),
            [KeyDef("⬆", SpecialKeys.shift, widthWeight: 1.5)]
                + chars("z", "x", "c", "v", "b", "n", "m")
                + [KeyDef("←", SpecialKeys.backspace, widthWeight: 1.5)],
            [
                KeyDef("?123", SpecialKeys.symbols, widthWeight: 1.5),
                KeyDef("🌐", SpecialKeys.globe, widthWeight: 1.0),
                KeyDef(",", Int(Character(",").unicodeScalars.first!.value), widthWeight: 1.0),
                KeyDef(" ", Int(Character(" ").unicodeScalars.first!.value), widthWeight: 4.0),
                KeyDef(".", Int(Character(".").unicodeScalars.first!.value), widthWeight: 1.0),
                KeyDef("↵", SpecialKeys.enter, widthWeight: 1.5),
            ],
        ]
    }

    public var symbols1Rows: [[KeyDef]] { EnglishLanguagePack().symbols1Rows }
    public var symbols2Rows: [[KeyDef]] { EnglishLanguagePack().symbols2Rows }

    public let longPressAccents: [Character: [Character]] = [
        "a": ["á", "à", "ä", "â", "ã"],
        "e": ["é", "è", "ê", "ë"],
        "i": ["í", "ì", "î", "ï"],
        "o": ["ó", "ò", "ô", "ö", "õ"],
        "u": ["ú", "ù", "û", "ü"],
        "?": ["¿"],
        "!": ["¡"],
    ]
}
