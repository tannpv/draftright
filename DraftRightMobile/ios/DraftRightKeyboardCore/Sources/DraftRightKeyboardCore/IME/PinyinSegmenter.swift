import Foundation

/// Splits a run-together pinyin string into syllables (#211, sentence-level
/// pinyin) — "woshi" → [wo, shi], "xianggang" → [xiang, gang]. Backtracking
/// longest-match (prefers longer syllables, so "xian" → [xian] not [xi, an]);
/// returns nil when no full segmentation exists. Mirror of the Kotlin
/// `PinyinSegmenter`.
public enum PinyinSegmenter {

    public static func segment(_ pinyin: String) -> [String]? {
        let s = Array(pinyin.lowercased())
        if s.isEmpty { return [] }
        var memo: [Int: [String]?] = [:]
        return segFrom(s, 0, &memo)
    }

    private static func segFrom(_ s: [Character], _ start: Int, _ memo: inout [Int: [String]?]) -> [String]? {
        if start == s.count { return [] }
        if let cached = memo[start] { return cached }
        let maxEnd = min(start + PinyinSyllables.maxLen, s.count)
        var end = maxEnd
        while end > start {
            let syllable = String(s[start..<end])
            if PinyinSyllables.isSyllable(syllable), let rest = segFrom(s, end, &memo) {
                let result = [syllable] + rest
                memo[start] = result
                return result
            }
            end -= 1
        }
        memo[start] = Optional<[String]>.none
        return nil
    }
}
