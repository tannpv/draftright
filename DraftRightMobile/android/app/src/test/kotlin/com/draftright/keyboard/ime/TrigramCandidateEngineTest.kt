package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class TrigramCandidateEngineTest {

    private val viWords = InMemoryWordList(
        words = listOf(
            "người" to 100,
            "ngoại" to 80,
            "ngon"  to 60,
            "ngạc"  to 40,
            "ngân"  to 30,
        ),
        bigrams = mapOf(
            "người" to mapOf("đẹp" to 20, "tốt" to 15, "yêu" to 10),
        ),
    )

    @Test
    fun `prefix completion ordered by frequency`() {
        val engine = TrigramCandidateEngine(viWords)
        val out = engine.suggest(composing = "ng", limit = 3)
        assertEquals(listOf("người", "ngoại", "ngon"), out.map { it.text })
    }

    @Test
    fun `empty composing returns next-word from bigram`() {
        val engine = TrigramCandidateEngine(viWords)
        val out = engine.suggest(composing = "", previousTokens = listOf("người"), limit = 5)
        assertEquals(listOf("đẹp", "tốt", "yêu"), out.map { it.text })
    }

    @Test
    fun `bigram boost reranks completions when last token is known`() {
        // Word list with two completions for "n" — both equally common — but
        // only one of them is a known successor of "người". The successor wins.
        val words = InMemoryWordList(
            words = listOf("nhanh" to 50, "nóng" to 50),
            bigrams = mapOf("người" to mapOf("nóng" to 5)),
        )
        val engine = TrigramCandidateEngine(words)
        val out = engine.suggest(composing = "n", previousTokens = listOf("người"), limit = 2)
        assertEquals("nóng", out.first().text)
    }

    @Test
    fun `unknown prefix yields empty list, no crash`() {
        val engine = TrigramCandidateEngine(viWords)
        assertTrue(engine.suggest("zzzz").isEmpty())
    }

    @Test
    fun `limit clamps result count`() {
        val engine = TrigramCandidateEngine(viWords)
        assertEquals(2, engine.suggest("ng", limit = 2).size)
    }

    @Test
    fun `case insensitive prefix match preserves original casing`() {
        val words = InMemoryWordList(words = listOf("Saigon" to 10))
        val engine = TrigramCandidateEngine(words)
        assertEquals("Saigon", engine.suggest("sai").first().text)
    }
}
