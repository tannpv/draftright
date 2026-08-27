package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/** JP engine offers the katakana reading as a candidate (#211/#212). TC: JPKATA-2 */
class KatakanaCandidateEngineTest {

    private val engine = KatakanaCandidateEngine(
        mapOf("かな" to listOf("仮名")),
    )

    private fun texts(reading: String) =
        engine.suggest(reading, emptyList(), 10).map { it.text }

    @Test fun `offers katakana alongside kanji and hiragana`() {
        val out = texts("かな")
        assertTrue("kanji present", out.contains("仮名"))
        assertTrue("katakana present", out.contains("カナ"))
        assertTrue("hiragana present", out.contains("かな"))
        // kanji first, katakana above the plain hiragana fallback.
        assertTrue(out.indexOf("カナ") < out.indexOf("かな"))
    }

    @Test fun `unknown reading still offers katakana + hiragana`() {
        val out = texts("こーひー")
        assertTrue(out.contains("コーヒー"))
        assertTrue(out.contains("こーひー"))
    }

    @Test fun `no duplicate katakana when reading has no hiragana`() {
        // Already katakana in → transliteration equals input → not added twice.
        assertEquals(listOf("カナ"), texts("カナ"))
    }

    @Test fun `empty composing yields nothing`() {
        assertTrue(texts("").isEmpty())
    }
}
