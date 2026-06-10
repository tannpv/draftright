import Foundation

/// Composer for Japanese: accumulates rōmaji and exposes the live kana as the
/// composing buffer (the candidate engine reads it for kanji conversion).
///
/// Only the rōmaji→kana transform is Japanese-specific; the keystroke/backspace/
/// commit/reset machinery + memoization come from `BufferingComposer` (Rule #1),
/// which a future PinyinComposer can reuse the same way.
public final class RomajiKanaComposer: BufferingComposer {
    public override func transform(_ raw: String) -> String {
        let c = RomajiComposer()
        c.feed(raw)
        return c.text()
    }
}
