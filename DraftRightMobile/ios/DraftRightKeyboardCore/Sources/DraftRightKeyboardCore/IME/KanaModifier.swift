import Foundation

/// The 小゛゜ modifier for Japanese flick (#212, phase 3): cycles the last kana
/// through its small/dakuten/handakuten variants — か→が→か, は→ば→ぱ→は,
/// つ→っ→づ→つ, や→ゃ→や. Cited linguistic constant, mirror of the Kotlin
/// `KanaModifier` (parity-guarded). Kana with no variants return unchanged.
public enum KanaModifier {

    private static let cycles: [[String]] = [
        ["あ", "ぁ"], ["い", "ぃ"], ["う", "ぅ", "ゔ"], ["え", "ぇ"], ["お", "ぉ"],
        ["か", "が"], ["き", "ぎ"], ["く", "ぐ"], ["け", "げ"], ["こ", "ご"],
        ["さ", "ざ"], ["し", "じ"], ["す", "ず"], ["せ", "ぜ"], ["そ", "ぞ"],
        ["た", "だ"], ["ち", "ぢ"], ["つ", "っ", "づ"], ["て", "で"], ["と", "ど"],
        ["は", "ば", "ぱ"], ["ひ", "び", "ぴ"], ["ふ", "ぶ", "ぷ"], ["へ", "べ", "ぺ"], ["ほ", "ぼ", "ぽ"],
        ["や", "ゃ"], ["ゆ", "ゅ"], ["よ", "ょ"],
        ["わ", "ゎ"],
    ]

    private static let next: [String: String] = {
        var m: [String: String] = [:]
        for cycle in cycles {
            for i in cycle.indices { m[cycle[i]] = cycle[(i + 1) % cycle.count] }
        }
        return m
    }()

    /// The next variant of `kana`, or `kana` unchanged if it has none.
    public static func cycle(_ kana: String) -> String { next[kana] ?? kana }
}
