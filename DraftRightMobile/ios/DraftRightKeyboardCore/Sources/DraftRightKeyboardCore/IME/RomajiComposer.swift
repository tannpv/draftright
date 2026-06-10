import Foundation

/// Deterministic rōmaji → hiragana composer (the offline front-half of Japanese
/// input). It accumulates typed rōmaji and converts it greedily to kana,
/// keeping any not-yet-resolvable tail as rōmaji. The resulting kana is what
/// the RIME engine converts to kanji candidates; on its own it already lets a
/// user type usable hiragana with no dictionary/network.
///
/// Rules handled: base syllables, y-glides (kya/sha/cho…), the irregulars
/// (shi/chi/tsu/fu + ja/ji/zu…), small tsu / sokuon (doubled consonant → っ),
/// and the moraic n (nn / n' / n+consonant → ん).
public final class RomajiComposer {
    private var kana = ""    // resolved kana so far
    private var pending = "" // unresolved rōmaji tail

    public init() {}

    /// Current composing text = resolved kana + unresolved rōmaji tail.
    /// A trailing lone "n" finalizes to the moraic ん so the kana is
    /// dictionary-lookable ("nihon" → にほん → 日本); without this it stayed
    /// literal ("にほn") and the candidate engine never matched. After an "nn"
    /// pair the ん is already emitted, so the leftover n is dropped (the doubled
    /// n is just the ん shortcut) rather than doubling to んん. A following key
    /// still re-binds it (raw "na" transforms fresh to な).
    public func text() -> String {
        if pending == "n" {
            return kana + (kana.hasSuffix("ん") ? "" : "ん")
        }
        return kana + pending
    }

    public func reset() {
        kana = ""
        pending = ""
    }

    /// Feed one or more rōmaji characters; returns the current composing text.
    @discardableResult
    public func feed(_ s: String) -> String {
        for ch in s.lowercased() { feedChar(ch) }
        return text()
    }

    private func feedChar(_ ch: Character) {
        pending.append(ch)
        resolve()
    }

    private func resolve() {
        // Greedily convert the pending buffer from the front.
        while !pending.isEmpty {
            let chars = Array(pending)
            // Sokuon: a doubled non-"n" consonant → っ + drop one.
            if chars.count >= 2, chars[0] == chars[1], Self.isConsonant(chars[0]), chars[0] != "n" {
                kana.append("っ")
                pending.removeFirst()
                continue
            }
            // Moraic n: "n" + a non-y consonant (incl. another "n") → ん, keeping
            // that consonant to start the next mora (so "konnichiwa" → こんにちわ,
            // "honda" → ほんだ). "n'" forces a standalone ん and consumes both.
            if chars[0] == "n", chars.count >= 2 {
                let second = chars[1]
                if second == "'" {
                    kana.append("ん")
                    pending.removeFirst(2)
                    continue
                }
                if Self.isConsonant(second), second != "y" {
                    kana.append("ん")
                    pending.removeFirst() // keep `second` to start the next mora
                    continue
                }
            }
            // Longest-match against the table (3 → 2 → 1 chars).
            var matched = false
            for len in stride(from: min(3, chars.count), through: 1, by: -1) {
                if let kanaUnit = Self.table[String(chars[0..<len])] {
                    kana.append(kanaUnit)
                    pending.removeFirst(len)
                    matched = true
                    break
                }
            }
            if matched { continue }
            // No match yet: if the buffer could still grow into a table key
            // (e.g. "k" → "ka", "ky" → "kya", "n" → "ni"), wait for more input.
            if Self.table.keys.contains(where: { $0.hasPrefix(pending) }) { break }
            // Otherwise it can never resolve — flush the leading char literally
            // (shown as typed) and retry the remainder.
            kana.append(chars[0])
            pending.removeFirst()
        }
    }

    private static func isConsonant(_ c: Character) -> Bool {
        return "bcdfghjklmnpqrstvwxyz".contains(c)
    }

    // Rōmaji → hiragana. Longest entries (3) first conceptually; lookup is by
    // exact key so order doesn't matter.
    static let table: [String: String] = [
        // vowels
        "a": "あ", "i": "い", "u": "う", "e": "え", "o": "お",
        // k / g
        "ka": "か", "ki": "き", "ku": "く", "ke": "け", "ko": "こ",
        "ga": "が", "gi": "ぎ", "gu": "ぐ", "ge": "げ", "go": "ご",
        "kya": "きゃ", "kyu": "きゅ", "kyo": "きょ",
        "gya": "ぎゃ", "gyu": "ぎゅ", "gyo": "ぎょ",
        // s / z
        "sa": "さ", "si": "し", "shi": "し", "su": "す", "se": "せ", "so": "そ",
        "za": "ざ", "zi": "じ", "ji": "じ", "zu": "ず", "ze": "ぜ", "zo": "ぞ",
        "sha": "しゃ", "shu": "しゅ", "sho": "しょ",
        "sya": "しゃ", "syu": "しゅ", "syo": "しょ",
        "ja": "じゃ", "ju": "じゅ", "jo": "じょ",
        // t / d
        "ta": "た", "ti": "ち", "chi": "ち", "tu": "つ", "tsu": "つ", "te": "て", "to": "と",
        "da": "だ", "di": "ぢ", "du": "づ", "de": "で", "do": "ど",
        "cha": "ちゃ", "chu": "ちゅ", "cho": "ちょ",
        "tya": "ちゃ", "tyu": "ちゅ", "tyo": "ちょ",
        // n
        "na": "な", "ni": "に", "nu": "ぬ", "ne": "ね", "no": "の",
        "nya": "にゃ", "nyu": "にゅ", "nyo": "にょ",
        // h / b / p
        "ha": "は", "hi": "ひ", "hu": "ふ", "fu": "ふ", "he": "へ", "ho": "ほ",
        "ba": "ば", "bi": "び", "bu": "ぶ", "be": "べ", "bo": "ぼ",
        "pa": "ぱ", "pi": "ぴ", "pu": "ぷ", "pe": "ぺ", "po": "ぽ",
        "hya": "ひゃ", "hyu": "ひゅ", "hyo": "ひょ",
        "bya": "びゃ", "byu": "びゅ", "byo": "びょ",
        "pya": "ぴゃ", "pyu": "ぴゅ", "pyo": "ぴょ",
        "fa": "ふぁ", "fi": "ふぃ", "fe": "ふぇ", "fo": "ふぉ",
        // m
        "ma": "ま", "mi": "み", "mu": "む", "me": "め", "mo": "も",
        "mya": "みゃ", "myu": "みゅ", "myo": "みょ",
        // y
        "ya": "や", "yu": "ゆ", "yo": "よ",
        // r
        "ra": "ら", "ri": "り", "ru": "る", "re": "れ", "ro": "ろ",
        "rya": "りゃ", "ryu": "りゅ", "ryo": "りょ",
        // w / standalone n
        "wa": "わ", "wo": "を", "nn": "ん",
        // punctuation
        "-": "ー", ",": "、", ".": "。",
    ]
}
