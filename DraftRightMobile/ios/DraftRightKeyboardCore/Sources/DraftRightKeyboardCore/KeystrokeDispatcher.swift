import Foundation

/// Abstraction over the text-insertion surface a keyboard writes to.
/// `UITextDocumentProxy` is adapted to this in the keyboard extension;
/// tests inject a spy so the outcome→call mapping is verified headlessly
/// (no simulator, no UIKit) via `swift test`.
public protocol KeyboardTextProxy: AnyObject {
    func insert(_ text: String)
    func deleteBackward()
    /// Show `text` as the in-flight composing (marked) region with the
    /// cursor at its end.
    func setComposing(_ text: String)
    /// Finalize / clear any composing region (UIKit `unmarkText`).
    func clearComposing()
}

/// Maps a `KeystrokeOutcome` to text-proxy calls. This is the exact seam
/// that produced the Tier β shipping bugs (composing region not cleared,
/// space replacing the marked word), so it is unit-tested directly.
public enum KeystrokeDispatcher {
    public static func apply(_ outcome: KeystrokeOutcome, to proxy: KeyboardTextProxy) {
        switch outcome {
        case .commit(let text):
            // Clear the marked region first, otherwise insertText replaces it.
            proxy.clearComposing()
            proxy.insert(text)
        case .composing(let text):
            proxy.setComposing(text)
        case .deleteOne:
            proxy.deleteBackward()
        case .noChange:
            // Composer emptied its buffer; the stale marked region must be
            // cleared or the user looks stuck on the first character.
            proxy.clearComposing()
        }
    }
}
