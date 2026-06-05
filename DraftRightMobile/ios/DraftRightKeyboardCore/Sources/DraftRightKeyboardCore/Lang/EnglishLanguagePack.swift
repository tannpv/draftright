import Foundation

public struct EnglishLanguagePack: LanguagePack {
    public let id = "en"
    public let displayName = "English"
    public let locale = Locale(identifier: "en")
    public let longPressAccents: [Character: [Character]] = [:]

    public init() {}

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
            chars("@", "#", "$", "%", "&", "-", "_", "+", "(", ")"),
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
