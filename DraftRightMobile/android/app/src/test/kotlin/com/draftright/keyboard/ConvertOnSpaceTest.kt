package com.draftright.keyboard

import com.draftright.keyboard.lang.ChineseLanguagePack
import com.draftright.keyboard.lang.EnglishLanguagePack
import com.draftright.keyboard.lang.JapaneseLanguagePack
import com.draftright.keyboard.lang.KoreanLanguagePack
import com.draftright.keyboard.lang.VietnameseLanguagePack
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Locks which input methods convert the composing reading to the top candidate
 * on space (#207 Phase 2, standard JP/ZH IME behavior): reading-conversion packs
 * (Japanese kana→kanji, Chinese pinyin→hanzi) convert; direct/prediction packs
 * (English, Vietnamese Telex, Korean Hangul) treat space as a literal space.
 * TC: JPSP-1..2
 */
class ConvertOnSpaceTest {

    @Test fun `reading-conversion packs convert on space`() {
        assertTrue("Japanese should convert kana->kanji on space", JapaneseLanguagePack.convertsOnSpace)
        assertTrue("Chinese should convert pinyin->hanzi on space", ChineseLanguagePack.convertsOnSpace)
    }

    @Test fun `direct and prediction packs keep space literal`() {
        assertFalse("English types directly", EnglishLanguagePack.convertsOnSpace)
        assertFalse("Vietnamese Telex commits the word, space is a space", VietnameseLanguagePack.convertsOnSpace)
        // Korean assembles Hangul syllables directly — no candidate conversion.
        assertFalse("Korean Hangul has no space conversion", KoreanLanguagePack.convertsOnSpace)
    }
}
