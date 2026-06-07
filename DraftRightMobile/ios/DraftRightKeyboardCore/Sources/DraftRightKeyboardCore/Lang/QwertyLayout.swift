import Foundation

/// Standard ASCII QWERTY key layout shared by every Latin/rōmaji pack that
/// doesn't need a bespoke arrangement (English, Japanese rōmaji, …).
///
/// Rule #1: the three big row tables live in ONE place instead of being copied
/// into each pack. Packs that need a different layout (e.g. French AZERTY)
/// simply don't use this.
public enum QwertyLayout {
    public static let alphaRows: [[KeyDef]] = [
        chars("q", "w", "e", "r", "t", "y", "u", "i", "o", "p"),
        chars("a", "s", "d", "f", "g", "h", "j", "k", "l"),
        [KeyDef("⬆", SpecialKeys.shift, widthWeight: 1.5)]
            + chars("z", "x", "c", "v", "b", "n", "m")
            + [KeyDef("←", SpecialKeys.backspace, widthWeight: 1.5)],
        bottomRow(layerKey: KeyDef("?123", SpecialKeys.symbols, widthWeight: 1.5)),
    ]

    public static let symbols1Rows: [[KeyDef]] = [
        chars("1", "2", "3", "4", "5", "6", "7", "8", "9", "0"),
        chars("@", "#", "$", "%", "&", "-", "+", "(", ")"),
        [KeyDef("#+=", SpecialKeys.symbols2, widthWeight: 1.5)]
            + chars("!", "\"", "'", ":", ";", "/", "?")
            + [KeyDef("←", SpecialKeys.backspace, widthWeight: 1.5)],
        bottomRow(layerKey: KeyDef("ABC", SpecialKeys.alpha, widthWeight: 1.5)),
    ]

    public static let symbols2Rows: [[KeyDef]] = [
        chars("~", "`", "|", "•", "√", "π", "÷", "×", "¶", "Δ"),
        chars("£", "€", "¥", "^", "[", "]", "{", "}"),
        [KeyDef("?123", SpecialKeys.symbols, widthWeight: 1.5)]
            + chars("©", "®", "™", "\\", "<", ">", "=")
            + [KeyDef("←", SpecialKeys.backspace, widthWeight: 1.5)],
        bottomRow(layerKey: KeyDef("ABC", SpecialKeys.alpha, widthWeight: 1.5)),
    ]

    /// The shared bottom row (layer-switch · globe · , · space · . · enter).
    private static func bottomRow(layerKey: KeyDef) -> [KeyDef] {
        [
            layerKey,
            KeyDef("🌐", SpecialKeys.globe, widthWeight: 1.0),
            KeyDef(",", keyCode(","), widthWeight: 1.0),
            KeyDef(" ", keyCode(" "), widthWeight: 4.0),
            KeyDef(".", keyCode("."), widthWeight: 1.0),
            KeyDef("↵", SpecialKeys.enter, widthWeight: 1.5),
        ]
    }
}
