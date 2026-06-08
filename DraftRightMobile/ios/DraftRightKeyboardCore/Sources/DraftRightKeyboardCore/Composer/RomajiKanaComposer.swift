import Foundation

/// Composer for Japanese: accumulates typed rōmaji and exposes the live kana
/// conversion as the composing buffer. The candidate engine
/// (`JapaneseDictionaryEngine`) reads `currentComposingText()` (= kana) and
/// offers kanji candidates; tapping one commits via the keyboard's candidate
/// handler.
///
/// Mirrors the Kotlin `RomajiKanaComposer` and the TelexComposer contract, so
/// the keystroke pipeline is unchanged (Rule #1) — the only Japanese-specific
/// bit is rōmaji→kana, delegated to the shared `RomajiComposer`.
public final class RomajiKanaComposer: Composer {

    private var raw = ""

    public init() {}

    private func kana() -> String {
        let c = RomajiComposer()
        c.feed(raw)
        return c.text()
    }

    public func onKey(_ char: Character) -> ComposeResult {
        if raw.count >= Self.maxLen {
            let committed = kana()
            raw = ""
            if char.isLetter {
                raw.append(char)
                return .composing(kana())
            }
            return .commit(committed + String(char))
        }

        if !char.isLetter {
            // Non-letter (space/punct/digit): commit the kana so far + the char.
            if raw.isEmpty { return .passThrough }
            let out = kana() + String(char)
            raw = ""
            return .commit(out)
        }

        raw.append(char)
        return .composing(kana())
    }

    public func onBackspace() -> ComposeResult {
        if raw.isEmpty { return .passThrough }
        raw.removeLast()
        return raw.isEmpty ? .consumed : .composing(kana())
    }

    public func reset() { raw = "" }

    public func currentComposingText() -> String { kana() }

    private static let maxLen = 48
}
