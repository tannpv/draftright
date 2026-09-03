package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * The IME asks its candidate engine for a correction rather than holding a
 * second copy of the dictionary (#207): the engine already owns the word list
 * and is cached per pack. Engines without a frequency dictionary (CJK reading
 * conversion) inherit the default "never correct".
 */
class CandidateEngineAutoCorrectTest {

    private val words = InMemoryWordList(listOf("không" to 668048, "khô" to 4000))

    @Test
    fun trigramEngineDelegatesToAutoCorrector() {
        val engine = TrigramCandidateEngine(words)
        assertEquals(AutoCorrector.correct("khôg", words), engine.autoCorrect("khôg"))
        assertEquals("không", engine.autoCorrect("khôg"))
    }

    @Test
    fun engineWithoutADictionaryNeverCorrects() {
        val engine = object : CandidateEngine {
            override fun suggest(composing: String, previousTokens: List<String>, limit: Int) =
                emptyList<Candidate>()
        }
        assertNull(engine.autoCorrect("khôg"))
    }
}
