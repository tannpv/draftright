import Foundation

/// Applying grammar suggestions to text — the pure logic behind
/// ``GrammarCheckView``, extracted so it is reusable and unit-testable without
/// the SwiftUI view (RULE #1). Mirrors Windows `GrammarFixer` and Linux
/// `grammar_fixer`, and the three are held in lockstep by the shared
/// golden-vector parity tests (#107).
///
/// LLM-reported offsets are UNRELIABLE — models count tokens or bytes and drift
/// by several characters, which splices suggestions into the middle of words
/// (BR#49). Every range is therefore re-resolved from the issue's `original`
/// CONTENT against the current text; the numeric offset only disambiguates
/// duplicate occurrences, where the nearest one wins (ties keep the earliest).
enum GrammarFix {

    /// Locates `issue.original` in `text`, preferring the occurrence nearest the
    /// LLM-claimed offset. Returns nil when the original no longer exists (a
    /// stale issue — e.g. one already removed by an overlapping fix).
    static func resolveRange(of issue: GrammarIssue, in text: String) -> Range<String.Index>? {
        guard !issue.original.isEmpty else { return nil }
        var candidates: [Range<String.Index>] = []
        var searchFrom = text.startIndex
        while let r = text.range(of: issue.original, range: searchFrom..<text.endIndex) {
            candidates.append(r)
            searchFrom = r.upperBound
        }
        guard !candidates.isEmpty else { return nil }
        let claimed = max(0, min(issue.offset, text.count))
        return candidates.min { a, b in
            abs(text.distance(from: text.startIndex, to: a.lowerBound) - claimed)
                < abs(text.distance(from: text.startIndex, to: b.lowerBound) - claimed)
        }
    }

    /// Apply one suggestion, returning the new text. Unchanged when the issue is
    /// stale (its `original` is no longer present).
    static func applyFix(_ text: String, _ issue: GrammarIssue) -> String {
        guard let range = resolveRange(of: issue, in: text) else { return text }
        var out = text
        out.replaceSubrange(range, with: issue.suggestion)
        return out
    }

    /// Apply every suggestion in order, re-resolving against the evolving text.
    /// Order no longer matters because ranges come from content, not offsets.
    static func fixAll(_ text: String, _ issues: [GrammarIssue]) -> String {
        var out = text
        for issue in issues {
            out = applyFix(out, issue)
        }
        return out
    }
}
