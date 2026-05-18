import Foundation

private func chars(_ labels: String...) -> [KeyDef] {
    labels.map { KeyDef($0, Int($0.unicodeScalars.first!.value)) }
}

public struct PortugueseLanguagePack: LanguagePack {
    public let id = "pt"
    public let displayName = "Português"
    public let locale = Locale(identifier: "pt")

    public init() {}

    public var alphaRows: [[KeyDef]] {
        [
            chars("q", "w", "e", "r", "t", "y", "u", "i", "o", "p"),
            chars("a", "s", "d", "f", "g", "h", "j", "k", "l", "ç"),
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
        "a": ["á", "â", "ã", "à", "ä"],
        "e": ["é", "ê", "ë"],
        "i": ["í", "î", "ï"],
        "o": ["ó", "ô", "õ", "ö"],
        "u": ["ú", "û", "ü"],
        "c": ["ç"],
    ]
}
