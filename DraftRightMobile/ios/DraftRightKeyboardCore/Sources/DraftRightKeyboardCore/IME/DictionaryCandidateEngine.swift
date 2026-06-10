import Foundation

/// Generic reading→candidates engine: looks the live composing string up in a
/// dictionary and offers the matches, with the reading itself as a fallback.
///
/// Reading-script conversion is the composer's job (RomajiKanaComposer produces
/// kana, PinyinComposer produces pinyin), so the SAME engine serves Japanese
/// (kana→kanji) and Chinese (pinyin→hanzi) — only the dictionary differs
/// (Rule #1).
public final class DictionaryCandidateEngine: CandidateEngine {

    /// reading → ranked candidates (best first).
    private let dictionary: [String: [String]]

    public init(dictionary: [String: [String]]) {
        self.dictionary = dictionary
    }

    public func suggest(composing: String, previousTokens: [String], limit: Int) -> [Candidate] {
        guard !composing.isEmpty else { return [] }
        var out: [Candidate] = []
        if let cands = dictionary[composing] {
            out += cands.map { Candidate(text: $0, display: $0) }
        }
        // The plain reading is always a valid commit (hiragana, or raw pinyin).
        if !out.contains(where: { $0.text == composing }) {
            out.append(Candidate(text: composing, display: composing))
        }
        return Array(out.prefix(limit))
    }
}
