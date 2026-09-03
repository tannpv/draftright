package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Exact-frequency lookup backing auto-correct (#207): "is this a real word,
 * and how common is it". Case-insensitive like prefixMatches/fuzzyMatches, so
 * a capitalised sentence-start isn't treated as an unknown word.
 */
class LanguageWordListFrequencyTest {

    private val list = InMemoryWordList(listOf("là" to 100, "anh" to 50, "Saigon" to 20))

    @Test
    fun knownWordReturnsFrequency() {
        assertEquals(100, list.frequencyOf("là"))
    }

    @Test
    fun unknownWordReturnsZero() {
        assertEquals(0, list.frequencyOf("xyz"))
    }

    @Test
    fun lookupIgnoresCase() {
        assertEquals(20, list.frequencyOf("saigon"))
        assertEquals(50, list.frequencyOf("ANH"))
    }

    @Test
    fun emptyTokenReturnsZero() {
        assertEquals(0, list.frequencyOf(""))
    }

    @Test
    fun duplicateEntriesKeepTheHighestFrequency() {
        val dupes = InMemoryWordList(listOf("ta" to 10, "ta" to 900))
        assertEquals(900, dupes.frequencyOf("ta"))
    }
}
