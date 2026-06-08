import Foundation

/// Shared skeleton for composers that accumulate raw input into a buffer and
/// expose a transformed "display" (composing) string — e.g. rōmaji→kana for
/// Japanese, pinyin for Chinese. Subclasses override only `transform`; the
/// keystroke/backspace/commit/reset machinery lives here once (Rule #1).
///
/// Mirrors the Kotlin BufferingComposer. Not used by TelexComposer (its
/// Vietnamese combining is per-keystroke, not a whole-buffer transform).
open class BufferingComposer: Composer {

    private var raw = ""
    private var cachedRaw: String?
    private var cachedDisplay = ""

    public init() {}

    /// Convert the accumulated raw input to the composing/display text.
    /// Subclasses MUST override.
    open func transform(_ raw: String) -> String { raw }

    /// Which characters are buffered as input. Default: letters.
    open func isInputChar(_ char: Character) -> Bool { char.isLetter }

    /// Max raw length before auto-committing, to bound the buffer.
    open var maxLen: Int { 48 }

    /// Memoized transform — recomputed only when the raw buffer changes.
    private func display() -> String {
        if cachedRaw != raw {
            cachedRaw = raw
            cachedDisplay = transform(raw)
        }
        return cachedDisplay
    }

    public func onKey(_ char: Character) -> ComposeResult {
        if raw.count >= maxLen {
            let committed = display()
            raw = ""
            if isInputChar(char) {
                raw.append(char)
                return .composing(display())
            }
            return .commit(committed + String(char))
        }
        if !isInputChar(char) {
            if raw.isEmpty { return .passThrough }
            let out = display() + String(char)
            raw = ""
            return .commit(out)
        }
        raw.append(char)
        return .composing(display())
    }

    public func onBackspace() -> ComposeResult {
        if raw.isEmpty { return .passThrough }
        raw.removeLast()
        return raw.isEmpty ? .consumed : .composing(display())
    }

    public func reset() {
        raw = ""
        cachedRaw = nil
        cachedDisplay = ""
    }

    public func currentComposingText() -> String { display() }
}
