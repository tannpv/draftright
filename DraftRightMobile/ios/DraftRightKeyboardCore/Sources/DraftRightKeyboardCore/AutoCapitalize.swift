import Foundation

/// Decides the auto-capitalization shift state, matching the stock Samsung /
/// iOS behaviour: engage SINGLE (capitalize-next) at the start of a field and
/// after sentence-ending punctuation, revert to OFF elsewhere, and never
/// override a user-engaged CAPS_LOCK.
///
/// Pure so the policy is unit-testable: the view supplies the text before the
/// cursor and whether the field requests sentence capitalization.
public enum AutoCapitalize {

    private static let terminators: Set<Character> = [".", "!", "?"]

    /// Whether the cursor sits at the start of a new sentence, given the text
    /// immediately before it. True at field/line start (empty) and after a
    /// sentence terminator followed by whitespace.
    public static func atSentenceStart(before: String?) -> Bool {
        let raw = before ?? ""
        let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty { return true }                       // field or line start
        guard let lastRaw = raw.last, lastRaw.isWhitespace else { return false } // cursor must follow a space
        guard let lastMeaningful = trimmed.last else { return false }
        return terminators.contains(lastMeaningful)
    }

    /// Resolve the shift state. CAPS_LOCK always wins; otherwise SINGLE only at
    /// a sentence start when the field opts into sentence capitalization.
    public static func resolve(current: ShiftState, sentenceCaps: Bool, atSentenceStart: Bool) -> ShiftState {
        if current == .capsLock { return .capsLock }
        return (sentenceCaps && atSentenceStart) ? .single : .off
    }
}
