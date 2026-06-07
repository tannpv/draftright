import Foundation

/// Suggestion engine for Latin-script languages — Vietnamese, English,
/// French, Spanish, German, Italian, Portuguese. Mirrors the Android
/// implementation in `keyboard.ime.TrigramCandidateEngine` 1:1 so both
/// platforms behave identically.
///
/// Algorithm (cheap, runs every keystroke):
///   1. Word completion — prefix-match the composing buffer against the
///      word list, ordered by frequency.
///   2. Next-word prediction — when `composing` is empty and we have at
///      least one `previousTokens` entry, look up `previousTokens.last`
///      in the bigram table and return the top successors.
///   3. Merge + truncate to `limit`.
///
/// Why a trigram and not a neural model: a static n-gram is two orders of
/// magnitude smaller (a few MB vs 100+) and runs in microseconds on every
/// keystroke without battery cost. The seam (`CandidateEngine`) means we
/// can swap to a small LM later without UI changes.
public final class TrigramCandidateEngine: CandidateEngine {

    private let wordList: LanguageWordList

    /// Bigram score weight relative to unigram frequency. Tuned to match
    /// the Android constant of the same name so candidate ordering is
    /// consistent across platforms.
    public static let bigramWeight: Int = 5

    public init(wordList: LanguageWordList) {
        self.wordList = wordList
    }

    public func suggest(composing: String, previousTokens: [String], limit: Int) -> [Candidate] {
        guard limit > 0 else { return [] }
        if composing.isEmpty {
            return nextWord(previousTokens: previousTokens, limit: limit)
        }
        return completions(prefix: composing, previousTokens: previousTokens, limit: limit)
    }

    public func close() { wordList.close() }

    // MARK: - Private

    private func completions(prefix: String, previousTokens: [String], limit: Int) -> [Candidate] {
        let matches = wordList.prefixMatches(prefix, limit: limit * 2)
        // Boost candidates that are valid successors of the last token.
        let successors: [String: Int]
        if let lastToken = previousTokens.last {
            successors = wordList.successors(lastToken)
        } else {
            successors = [:]
        }
        let ranked = matches.sorted { a, b in
            let aBoost = successors[a.word] ?? 0
            let bBoost = successors[b.word] ?? 0
            let aScore = a.freq + aBoost * Self.bigramWeight
            let bScore = b.freq + bBoost * Self.bigramWeight
            return aScore > bScore
        }
        return ranked.prefix(limit).map { Candidate(text: $0.word) }
    }

    private func nextWord(previousTokens: [String], limit: Int) -> [Candidate] {
        guard let last = previousTokens.last else { return [] }
        let successors = wordList.successors(last)
        if successors.isEmpty { return [] }
        return successors
            .sorted { $0.value > $1.value }
            .prefix(limit)
            .map { Candidate(text: $0.key) }
    }
}
