import Foundation

/// Extracts the recent committed words before the cursor to feed
/// `CandidateEngine.suggest`'s `previousTokens` (next-word prediction + bigram
/// boosting). Pure string logic so it is unit-tested without a text proxy, and
/// shared by every language pack — not Vietnamese-specific.
///
/// Mirror of Kotlin `keyboard.PreviousTokens`. The live composing word is
/// excluded: it is the word being typed now, not a *previous* token, and
/// including it would make the engine predict the current word as its own
/// successor.
public enum PreviousTokens {

    /// Trigram context depth — the engine only uses the most-recent token today,
    /// but keeping a few gives headroom for a longer context model behind the
    /// same seam without touching callers. Mirrors Android `MAX`.
    public static let max = 3

    public static func fromTextBeforeCursor(
        _ textBeforeCursor: String?,
        composing: String,
        max: Int = PreviousTokens.max
    ) -> [String] {
        guard var s = textBeforeCursor else { return [] }
        // Strip the live composing suffix — but only when it really is the tail,
        // so a composer that diverged from the field can't chop committed text.
        if !composing.isEmpty && s.hasSuffix(composing) {
            s = String(s.dropLast(composing.count))
        }
        let tokens = s
            .split(whereSeparator: { $0.isWhitespace })
            .map { trimNonLetters($0) }
            .filter { !$0.isEmpty }
        return Array(tokens.suffix(max))
    }

    /// Drop leading/trailing non-letter characters (attached punctuation), Unicode
    /// aware so Vietnamese diacritics count as letters. Matches Kotlin's
    /// `trim { !it.isLetter() }`.
    private static func trimNonLetters(_ s: Substring) -> String {
        var chars = Array(s)
        while let first = chars.first, !first.isLetter { chars.removeFirst() }
        while let last = chars.last, !last.isLetter { chars.removeLast() }
        return String(chars)
    }
}
