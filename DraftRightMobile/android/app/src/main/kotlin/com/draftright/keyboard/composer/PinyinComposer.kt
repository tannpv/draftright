package com.draftright.keyboard.composer

/**
 * Composer for Chinese pinyin: buffers the typed pinyin and shows it as-is in
 * the composing region; the candidate engine (DictionaryCandidateEngine with a
 * pinyin→hanzi dictionary) turns it into Hanzi candidates.
 *
 * Unlike Japanese (rōmaji→kana) the composing display IS the raw pinyin, so the
 * transform is identity — all it needs from [BufferingComposer] is the buffer +
 * commit/backspace machinery.
 */
class PinyinComposer : BufferingComposer() {
    override fun transform(raw: String): String = raw
}
