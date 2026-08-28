import Foundation

/// Composer for Japanese **flick** input (#212): the flick keyboard emits kana
/// directly (romaji→kana already happened in the finger's flick), so the composer
/// just buffers the kana and shows them as the composing text — the identity
/// transform. The kana buffer then drives the same kana→kanji candidate engine
/// that the rōmaji path uses (Rule #1: only the input surface differs).
///
/// Mirrors the Kotlin `KanaComposer`. Contrast `RomajiKanaComposer`, which
/// transforms buffered rōmaji into kana.
public final class KanaComposer: BufferingComposer {

    public override func transform(_ raw: String) -> String { raw }

    /// Buffer every character the flick keyboard produces: kana are letters, but
    /// ー (chōonpu) and 〜 are not, so accept them explicitly. Space / punctuation
    /// routing is handled by the controller before it ever reaches here.
    public override func isInputChar(_ char: Character) -> Bool {
        char.isLetter || char == "ー" || char == "〜"
    }
}
