package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * The engine looks the composing reading up directly (script conversion is the
 * composer's job). Verified with both a kana dict (Japanese) and a pinyin dict
 * (Chinese) to lock the shared behaviour.
 */
class DictionaryCandidateEngineTest {

    private val kana = mapOf("にほんご" to listOf("日本語"), "かんじ" to listOf("漢字", "幹事"))
    private val pinyin = mapOf("nihao" to listOf("你好"), "zhongwen" to listOf("中文", "中問"))

    @Test fun `kana reading returns ranked kanji plus fallback`() {
        val c = DictionaryCandidateEngine(kana).suggest("にほんご")
        assertEquals("日本語", c.first().text)
        assertTrue(c.any { it.text == "にほんご" })
    }

    @Test fun `pinyin reading returns hanzi`() {
        val c = DictionaryCandidateEngine(pinyin).suggest("zhongwen")
        assertEquals(listOf("中文", "中問"), c.take(2).map { it.text })
    }

    @Test fun `unknown reading offers the reading itself`() {
        assertEquals(listOf("sora"), DictionaryCandidateEngine(kana).suggest("sora").map { it.text })
    }

    @Test fun `empty input no candidates`() {
        assertTrue(DictionaryCandidateEngine(kana).suggest("").isEmpty())
    }
}
