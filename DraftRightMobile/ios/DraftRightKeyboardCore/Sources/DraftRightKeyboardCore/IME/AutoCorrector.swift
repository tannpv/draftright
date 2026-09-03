import Foundation

/// Pure typo→correction decision for auto-correct-on-space (#207): given the
/// token the user just finished, return the word to commit instead, or `nil`
/// to leave it exactly as typed.
///
/// Deliberately conservative — a wrong correction is far more annoying than a
/// missed one, so it only fires when the token is not itself a word, has a
/// single candidate within `maxEdits`, and that candidate is both common and
/// clearly ahead of its runner-up.
///
/// Reuses `LanguageWordList.fuzzyMatches` for edit distance; there must never
/// be a second Levenshtein in this codebase. That also sets the limit of what
/// gets corrected: plain Levenshtein counts a transposition ("khôgn") as two
/// edits, so swapped letters are out of reach until the shared distance
/// function grows a transposition case on both platforms.
///
/// Mirror of Kotlin `AutoCorrector` — the thresholds below are asserted equal
/// by `scripts/check-autocorrect-consts-parity.py`, and both sides run the
/// shared `parity/autocorrect-vectors.json` cases.
public enum AutoCorrector {
    /// Only single-typo slips are corrected; 2+ edits are guesswork.
    public static let maxEdits = 1

    /// A candidate rarer than this isn't worth overriding the user for.
    public static let minConfidenceFreq = 500

    /// The winner must be this many times more frequent than the runner-up.
    public static let minConfidenceMargin = 4

    /// The corrected word for `token`, or `nil` to leave it as typed.
    /// Casing of `token` is preserved: a leading capital carries over to the
    /// correction, and anything less regular (ALL CAPS, mIxEd) is left alone
    /// because it's more likely an acronym or a name than a typo.
    public static func correct(_ token: String, _ words: LanguageWordList) -> String? {
        if token.isEmpty || token.contains(where: { !$0.isLetter }) { return nil }
        let lower = token.lowercased()
        let capitalized = token == capitalize(lower)
        if token != lower && !capitalized { return nil }

        if words.frequencyOf(lower) > 0 { return nil } // already a real word
        let candidates = words.fuzzyMatches(lower, maxEdits: maxEdits, limit: 2)
        guard let top = candidates.first else { return nil }
        if top.freq < minConfidenceFreq { return nil }
        let runnerUp = candidates.count > 1 ? candidates[1].freq : 0
        if top.freq < runnerUp * minConfidenceMargin { return nil } // too close to call

        return capitalized ? capitalize(top.word) : top.word
    }

    /// Uppercases only the first character — `String.capitalized` would also
    /// lowercase the rest and split on word boundaries, which a single token
    /// must not undergo.
    private static func capitalize(_ word: String) -> String {
        guard let first = word.first else { return word }
        return String(first).uppercased() + word.dropFirst()
    }
}
