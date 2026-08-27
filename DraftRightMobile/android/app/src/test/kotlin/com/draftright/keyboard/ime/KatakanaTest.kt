package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Test

/** Hiragana→katakana transliteration (#211/#212). TC: JPKATA-1 */
class KatakanaTest {

    @Test fun `converts gojuon`() {
        assertEquals("カナ", Katakana.fromHiragana("かな"))
        assertEquals("コーヒー", Katakana.fromHiragana("こーひー")) // ー passes through
        assertEquals("ヴ", Katakana.fromHiragana("ゔ"))            // U+3094 → U+30F4
    }

    @Test fun `small and dakuten kana convert`() {
        assertEquals("ッ", Katakana.fromHiragana("っ"))
        assertEquals("ガ", Katakana.fromHiragana("が"))
        assertEquals("パ", Katakana.fromHiragana("ぱ"))
        assertEquals("ャ", Katakana.fromHiragana("ゃ"))
    }

    @Test fun `non-hiragana passes through unchanged`() {
        assertEquals("", Katakana.fromHiragana(""))
        assertEquals("abc", Katakana.fromHiragana("abc"))
        assertEquals("日本", Katakana.fromHiragana("日本")) // kanji untouched
        assertEquals("カ", Katakana.fromHiragana("カ"))     // already katakana
    }
}
