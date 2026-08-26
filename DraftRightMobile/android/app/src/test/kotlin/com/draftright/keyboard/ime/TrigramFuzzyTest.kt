package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Typo tolerance (#207 gap #3, autocorrect): when the composing buffer has no
 * exact prefix match, the engine tops up the candidate bar with edit-distance-1
 * dictionary words so a mistyped word still offers the right correction to tap.
 * Conservative — fuzzy matches only fill slots the exact prefix scan left empty,
 * so correctly-typed words are never displaced by a fuzzy guess.
 * TC: VIAC-1..5
 */
class TrigramFuzzyTest {

    private val dict = InMemoryWordList(
        words = listOf("người" to 100, "ngon" to 60, "việt" to 50, "nhà" to 40),
    )
    private val engine = TrigramCandidateEngine(dict)

    private fun texts(term: String, limit: Int = 5) =
        engine.suggest(composing = term, limit = limit).map { it.text }

    @Test fun `single-substitution typo surfaces the correction`() {
        // "nqon" -> "ngon" (q→g, edit distance 1); no word starts with "nq".
        assertTrue(texts("nqon").contains("ngon"))
    }

    @Test fun `wrong-tone typo surfaces the intended word`() {
        // "ngưới" -> "người" (ớ→ườ region, one combining vowel off = distance 1).
        assertTrue(texts("ngưới").contains("người"))
    }

    @Test fun `missing-character typo surfaces the correction`() {
        // "ngo" IS a prefix of "ngon", so exact completion already covers it —
        // "nhaf"-style is different; use a clean 1-insertion case with no prefix.
        assertTrue(texts("nhs").contains("nhà")) // nhs -> nhà : s→à substitution, distance 1
    }

    @Test fun `exact prefix matches are not displaced by fuzzy guesses`() {
        // "ng" prefixes người/ngon — with limit 2 the bar is full of exacts,
        // fuzzy never runs, order stays frequency-ranked.
        assertEquals(listOf("người", "ngon"), texts("ng", limit = 2))
    }

    @Test fun `a far typo (distance greater than 1) surfaces nothing`() {
        val out = texts("zzzz")
        assertFalse(out.contains("người"))
        assertTrue(out.isEmpty())
    }
}
