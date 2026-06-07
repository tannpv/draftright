package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Kotlin parity for JapaneseDictLoader — mirrors the Swift JapaneseDictLoaderTests.
 * Uses the pure-String parse() overload so no disk I/O needed in unit tests.
 */
class JapaneseDictLoaderTest {

    @Test fun `basic reading to kanji`() {
        val d = JapaneseDictLoader.parse("にほんご\t日本語\nかんじ\t漢字,幹事")
        assertEquals(listOf("日本語"), d["にほんご"])
        assertEquals(listOf("漢字", "幹事"), d["かんじ"])
    }

    @Test fun `comments and blank lines skipped`() {
        val d = JapaneseDictLoader.parse("# header\n\nわたし\t私\n")
        assertEquals(1, d.size)
        assertEquals(listOf("私"), d["わたし"])
    }

    @Test fun `rank order preserved`() {
        val d = JapaneseDictLoader.parse("き\t木,気,来")
        assertEquals(listOf("木", "気", "来"), d["き"])
    }

    @Test fun `malformed line skipped`() {
        val d = JapaneseDictLoader.parse("noTab\nき\t木")
        assertEquals(1, d.size)
    }
}
