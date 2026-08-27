import Foundation

/// Japanese candidate engine (#211/#212): everything the shared
/// `DictionaryCandidateEngine` does (kana→kanji lookup + plain-hiragana
/// fallback), PLUS the **katakana** form of the reading (かな → カナ) — the
/// standard way every JP IME lets users write loanwords and names. Wrapping the
/// shared engine rather than editing it keeps Chinese (which uses the same
/// `DictionaryCandidateEngine`) untouched — Rule #1. Mirror of the Kotlin
/// `KatakanaCandidateEngine`.
public final class KatakanaCandidateEngine: CandidateEngine {

    private let base: DictionaryCandidateEngine

    public init(dictionary: [String: [String]]) {
        self.base = DictionaryCandidateEngine(dictionary: dictionary)
    }

    public func suggest(composing: String, previousTokens: [String], limit: Int) -> [Candidate] {
        guard !composing.isEmpty else { return [] }
        let baseCands = base.suggest(composing: composing, previousTokens: previousTokens, limit: limit)
        let katakana = Katakana.fromHiragana(composing)
        // Nothing to convert (no hiragana in the buffer) or already offered.
        if katakana == composing || baseCands.contains(where: { $0.text == katakana }) {
            return baseCands
        }
        // Slot katakana just above the plain-hiragana fallback (kanji stay on
        // top, hiragana stays last) — the ordering real JP IMEs use.
        var out = baseCands
        let candidate = Candidate(text: katakana, display: katakana)
        if let readingIdx = out.firstIndex(where: { $0.text == composing }) {
            out.insert(candidate, at: readingIdx)
        } else {
            out.append(candidate)
        }
        return Array(out.prefix(limit))
    }
}
