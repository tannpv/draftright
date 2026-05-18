import Foundation

private func chars(_ labels: String...) -> [KeyDef] {
    labels.map { KeyDef($0, Int($0.unicodeScalars.first!.value)) }
}

public struct GermanLanguagePack: LanguagePack {
    public let id = "de"
    public let displayName = "Deutsch"
    public let locale = Locale(identifier: "de")

    public init() {}

    public var alphaRows: [[KeyDef]] {
        [
            chars("q", "w", "e", "r", "t", "z", "u", "i", "o", "p", "ü"),
            chars("a", "s", "d", "f", "g", "h", "j", "k", "l", "ö", "ä"),
            [KeyDef("⬆", SpecialKeys.shift, widthWeight: 1.5)]
                + chars("y", "x", "c", "v", "b", "n", "m", "ß")
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
        "a": ["à", "á", "â", "ã"],
        "e": ["é", "è", "ê", "ë"],
        "i": ["í", "ì", "î", "ï"],
        "o": ["ó", "ò", "ô", "õ"],
        "u": ["ú", "ù", "û"],
    ]
}
