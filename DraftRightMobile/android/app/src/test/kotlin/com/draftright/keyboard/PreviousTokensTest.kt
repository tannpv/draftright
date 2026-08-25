package com.draftright.keyboard

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Locks the n-gram context extraction that feeds next-word prediction +
 * bigram boosting (the previously-dead `previousTokens` path in
 * TrigramCandidateEngine). Pure string logic — no InputConnection needed.
 * TC: VISUG-1..4
 */
class PreviousTokensTest {

    @Test fun `extracts committed words before cursor, order preserved`() {
        assertEquals(listOf("xin", "chào"), PreviousTokens.fromTextBeforeCursor("xin chào ", ""))
    }

    @Test fun `excludes the live composing word (its suffix is stripped)`() {
        // The cursor sits inside "viên" being typed — it's the current word, not
        // a previous token, so bigram context is the three words before it.
        assertEquals(
            listOf("tôi", "là", "sinh"),
            PreviousTokens.fromTextBeforeCursor("tôi là sinh viên", "viên"),
        )
    }

    @Test fun `caps are preserved (engine lowercases on lookup, not here)`() {
        assertEquals(listOf("Xin", "Chào"), PreviousTokens.fromTextBeforeCursor("Xin Chào ", ""))
    }

    @Test fun `depth is capped at MAX most-recent tokens`() {
        val out = PreviousTokens.fromTextBeforeCursor("a b c d e ", "")
        assertEquals(PreviousTokens.MAX, out.size)
        assertEquals(listOf("c", "d", "e"), out) // assumes MAX == 3
    }

    @Test fun `trailing punctuation is trimmed per token`() {
        assertEquals(listOf("Chào"), PreviousTokens.fromTextBeforeCursor("Chào,", ""))
    }

    @Test fun `empty and whitespace-only yield no tokens`() {
        assertEquals(emptyList<String>(), PreviousTokens.fromTextBeforeCursor("", ""))
        assertEquals(emptyList<String>(), PreviousTokens.fromTextBeforeCursor("   ", ""))
        assertEquals(emptyList<String>(), PreviousTokens.fromTextBeforeCursor(null, ""))
    }

    @Test fun `composing that is not actually the suffix is not stripped`() {
        // Defensive: if the composer buffer diverged from the field, don't chop
        // real committed text — treat all of it as previous tokens.
        assertEquals(listOf("bún", "bò"), PreviousTokens.fromTextBeforeCursor("bún bò", "phở"))
    }
}
