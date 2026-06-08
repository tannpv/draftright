import Foundation

/// Composer for Chinese pinyin: buffers the typed pinyin and shows it as-is in
/// the composing region; the candidate engine (DictionaryCandidateEngine with a
/// pinyin→hanzi dictionary) turns it into Hanzi candidates.
///
/// The composing display IS the raw pinyin, so the transform is identity (the
/// base default) — all it needs is the buffer + commit/backspace machinery.
public final class PinyinComposer: BufferingComposer {
    public override func transform(_ raw: String) -> String { raw }
}
