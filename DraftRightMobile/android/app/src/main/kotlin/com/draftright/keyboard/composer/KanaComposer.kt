package com.draftright.keyboard.composer

/**
 * Composer for Japanese **flick** input (#212): the flick keyboard emits kana
 * directly (romaji→kana already happened in the finger's flick), so the composer
 * just buffers the kana and shows them as the composing text — the identity
 * transform. The kana buffer then drives the same kana→kanji candidate engine
 * that the rōmaji path uses (RULE #1: only the input surface differs).
 *
 * Contrast [RomajiKanaComposer], which transforms buffered rōmaji into kana.
 */
class KanaComposer : BufferingComposer() {

    override fun transform(raw: String): String = raw

    /**
     * Buffer every character the flick keyboard produces: kana are letters, but
     * ー (chōonpu) and 〜 are not, so accept them explicitly. Space / punctuation
     * routing is handled by the IME before it ever reaches here.
     */
    override fun isInputChar(char: Char): Boolean =
        char.isLetter() || char == 'ー' || char == '〜'
}
