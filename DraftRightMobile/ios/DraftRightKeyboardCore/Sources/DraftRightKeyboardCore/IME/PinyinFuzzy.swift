import Foundation

/// Fuzzy-pinyin folding (#211): collapses the pinyin distinctions many speakers
/// don't reliably produce — retroflex↔dental initials (zh/ch/sh → z/c/s) and the
/// nasal finals (ang/eng/ing → an/en/in) — so "zongguo" finds 中国 and "si" finds
/// 是. Applied to both dictionary keys and the query. Mirror of the Kotlin
/// `PinyinFuzzy`.
public enum PinyinFuzzy {

    public static func fold(_ pinyin: String) -> String {
        var s = pinyin.lowercased()
        s = s.replacingOccurrences(of: "zh", with: "z")
            .replacingOccurrences(of: "ch", with: "c")
            .replacingOccurrences(of: "sh", with: "s")
        s = s.replacingOccurrences(of: "ang", with: "an")
            .replacingOccurrences(of: "eng", with: "en")
            .replacingOccurrences(of: "ing", with: "in")
        return s
    }
}
