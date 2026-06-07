package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Kotlin parity for the Japanese kana→kanji engine (mirrors the Swift
 * JapaneseDictionaryEngineTests). Proves the dictionary path works through the
 * shared CandidateEngine seam + RomajiComposer — no native code.
 */
class JapaneseDictionaryEngineTest {

    private val dict = mapOf(
        "にほん" to listOf("日本"),
        "にほんご" to listOf("日本語"),
        "かんじ" to listOf("漢字", "幹事"),
    )

    @Test fun `romaji converts to ranked kanji`() {
        val c = JapaneseDictionaryEngine(dict).suggest("nihongo")
        assertEquals("日本語", c.first().text)
        assertTrue(c.any { it.text == "にほんご" })
    }

    @Test fun `multiple kanji for one reading keep rank`() {
        val c = JapaneseDictionaryEngine(dict).suggest("kanji")
        assertEquals(listOf("漢字", "幹事"), c.take(2).map { it.text })
    }

    @Test fun `unknown reading still offers kana`() {
        assertEquals(listOf("そら"), JapaneseDictionaryEngine(dict).suggest("sora").map { it.text })
    }

    @Test fun `empty input no candidates`() {
        assertTrue(JapaneseDictionaryEngine(dict).suggest("").isEmpty())
    }
}
