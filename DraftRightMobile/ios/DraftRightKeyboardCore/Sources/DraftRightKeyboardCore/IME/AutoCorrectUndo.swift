import Foundation

/// One-shot undo for an auto-correction (#207). Auto-correct is only acceptable
/// if it is trivially reversible, so the correction arms this and the *next*
/// backspace consumes it — putting the typed word back instead of deleting a
/// character.
///
/// Pure state, no `UITextDocumentProxy`: the keyboard owns the text edits, this
/// owns only the decision. Mirror of Kotlin `AutoCorrectUndo`.
public final class AutoCorrectUndo {
    private var original: String?

    /// The word the correction committed, `nil` when nothing is armed.
    public private(set) var corrected: String?

    public init() {}

    /// Remember that `original` was committed as `corrected`.
    public func arm(original: String, corrected: String) {
        self.original = original
        self.corrected = corrected
    }

    /// Whether the undo is still live: `beforeCursor` (the field text in front
    /// of the cursor) still ends with the correction plus the space the
    /// correcting keystroke appended. One place owns this rule so the revert
    /// path and every disarm chokepoint can't drift apart.
    public func isLive(beforeCursor: String?) -> Bool {
        guard let corrected else { return false }
        return beforeCursor?.hasSuffix(corrected + " ") == true
    }

    /// The word to restore, disarming in the same step, or `nil` when nothing
    /// is armed (so the caller falls through to a normal backspace).
    public func consume() -> String? {
        let pending = original
        disarm()
        return pending
    }

    /// Forget the pending correction.
    public func disarm() {
        original = nil
        corrected = nil
    }
}
