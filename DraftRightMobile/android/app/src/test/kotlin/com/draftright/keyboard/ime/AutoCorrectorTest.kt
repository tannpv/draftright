package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * Auto-correct-on-space decision rules (#207). The dictionary here is a
 * miniature of the shipped VI list — real frequencies, few enough words to
 * reason about which candidate wins.
 */
class AutoCorrectorTest {

    private val words = InMemoryWordList(
        listOf("không" to 668048, "khô" to 4000, "anh" to 469245, "ảnh" to 12000),
    )

    @Test
    fun typoIsCorrectedToTheCommonNeighbour() {
        assertEquals("không", AutoCorrector.correct("khôg", words))
    }

    @Test
    fun realWordIsNeverTouched() {
        assertNull(AutoCorrector.correct("anh", words))
    }

    @Test
    fun wordWithNoNeighbourWithinOneEditIsLeftAlone() {
        assertNull(AutoCorrector.correct("zzzz", words))
    }

    @Test
    fun emptyTokenIsLeftAlone() {
        assertNull(AutoCorrector.correct("", words))
    }

    @Test
    fun tokenWithNonLettersIsLeftAlone() {
        assertNull(AutoCorrector.correct("kh1", words))
        assertNull(AutoCorrector.correct("anh's", words))
    }

    @Test
    fun rareCandidateIsNotConfidentEnough() {
        val rare = InMemoryWordList(listOf("khô" to AutoCorrector.MIN_CONFIDENCE_FREQ - 1))
        assertNull(AutoCorrector.correct("khù", rare))
    }

    @Test
    fun ambiguousCandidatesAreLeftAlone() {
        // "ta"/"tô" are both one edit from "tá" and comparably common — with no
        // clear winner, changing the user's word is worse than leaving it.
        val rivals = InMemoryWordList(listOf("ta" to 342219, "tô" to 300000))
        assertNull(AutoCorrector.correct("tá", rivals))
    }

    @Test
    fun leadingCapitalIsPreservedOnTheCorrection() {
        assertEquals("Không", AutoCorrector.correct("Khôg", words))
    }

    @Test
    fun acronymsAndMixedCaseAreLeftAlone() {
        assertNull(AutoCorrector.correct("KHÔG", words))
        assertNull(AutoCorrector.correct("kHôg", words))
    }
}
