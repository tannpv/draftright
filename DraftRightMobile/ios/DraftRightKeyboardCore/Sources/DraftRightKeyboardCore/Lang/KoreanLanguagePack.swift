import Foundation

/// Korean (한국어) pack — dubeolsik (2-set) jamo layout. Keys carry the jamo;
/// `HangulComposer` assembles them into syllable blocks live. No candidate bar
/// (Korean is deterministic composition). Mirrors the Kotlin KoreanLanguagePack.
///
/// Long-press a basic consonant/vowel for its tense/extra form (ㅂ→ㅃ, ㄱ→ㄲ, …).
public struct KoreanLanguagePack: LanguagePack {
    public let id = "ko"
    public let displayName = "한국어"
    public let locale = Locale(identifier: "ko")

    public let longPressAccents: [Character: [Character]] = [
        "ㅂ": ["ㅃ"], "ㅈ": ["ㅉ"], "ㄷ": ["ㄸ"], "ㄱ": ["ㄲ"], "ㅅ": ["ㅆ"],
        "ㅐ": ["ㅒ"], "ㅔ": ["ㅖ"],
    ]

    public init() {}

    public func makeComposer() -> Composer? { HangulComposer() }

    private func jamo(_ labels: String...) -> [KeyDef] {
        labels.map { KeyDef($0, keyCode($0)) }
    }

    public var alphaRows: [[KeyDef]] {
        [
            jamo("ㅂ", "ㅈ", "ㄷ", "ㄱ", "ㅅ", "ㅛ", "ㅕ", "ㅑ", "ㅐ", "ㅔ"),
            jamo("ㅁ", "ㄴ", "ㅇ", "ㄹ", "ㅎ", "ㅗ", "ㅓ", "ㅏ", "ㅣ"),
            [KeyDef("⬆", SpecialKeys.shift, widthWeight: 1.5)]
                + jamo("ㅋ", "ㅌ", "ㅊ", "ㅍ", "ㅠ", "ㅜ", "ㅡ")
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

    public var symbols1Rows: [[KeyDef]] { QwertyLayout.symbols1Rows }
    public var symbols2Rows: [[KeyDef]] { QwertyLayout.symbols2Rows }
}
