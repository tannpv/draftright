import Foundation

/// The Japanese 12-key flick (フリック) kana map (#212): each key is a gojūon row,
/// a flick picks the vowel — tap=あ, ←=い, ↑=う, →=え, ↓=お. A linguistic
/// constant, one source of truth, mirror of the Kotlin `FlickLayout`
/// (parity-guarded by scripts/check-flick-layout-parity.py). The resolved kana
/// feeds the existing kana→kanji engine unchanged.
public enum FlickLayout {

    /// Row-head kana (the key's tap output) → direction → kana.
    public static let rows: [String: [FlickDirection: String]] = [
        "あ": gojuon("あ", "い", "う", "え", "お"),
        "か": gojuon("か", "き", "く", "け", "こ"),
        "さ": gojuon("さ", "し", "す", "せ", "そ"),
        "た": gojuon("た", "ち", "つ", "て", "と"),
        "な": gojuon("な", "に", "ぬ", "ね", "の"),
        "は": gojuon("は", "ひ", "ふ", "へ", "ほ"),
        "ま": gojuon("ま", "み", "む", "め", "も"),
        // や row has only 3 kana: tap や, up ゆ, down よ.
        "や": [.tap: "や", .up: "ゆ", .down: "よ"],
        "ら": gojuon("ら", "り", "る", "れ", "ろ"),
        // わ row is special: tap わ, ← を, ↑ ん, → ー (chōonpu), ↓ 〜.
        "わ": [.tap: "わ", .left: "を", .up: "ん", .right: "ー", .down: "〜"],
    ]

    public static func kanaFor(_ rowHead: String, _ direction: FlickDirection) -> String? {
        rows[rowHead]?[direction]
    }

    private static func gojuon(_ a: String, _ i: String, _ u: String, _ e: String, _ o: String) -> [FlickDirection: String] {
        [.tap: a, .left: i, .up: u, .right: e, .down: o]
    }
}
