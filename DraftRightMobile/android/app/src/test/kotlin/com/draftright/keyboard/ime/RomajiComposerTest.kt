package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Test

class RomajiComposerTest {
    private fun kana(romaji: String): String = RomajiComposer().feed(romaji)

    @Test fun basicWords() {
        assertEquals("にほんご", kana("nihongo"))
        assertEquals("すし", kana("sushi"))
        assertEquals("ときょ", kana("tokyo"))
        assertEquals("こんにちわ", kana("konnichiwa"))
    }

    @Test fun irregulars() {
        assertEquals("し", kana("shi"))
        assertEquals("ち", kana("chi"))
        assertEquals("つ", kana("tsu"))
        assertEquals("ふ", kana("fu"))
        assertEquals("じ", kana("ji"))
    }

    @Test fun yGlides() {
        assertEquals("きゃ", kana("kya"))
        assertEquals("しゃ", kana("sha"))
        assertEquals("ちょ", kana("cho"))
        assertEquals("りゅ", kana("ryu"))
    }

    @Test fun sokuon() {
        assertEquals("きっと", kana("kitto"))
        assertEquals("きっぷ", kana("kippu"))
    }

    @Test fun moraicN() {
        assertEquals("んn", kana("nn"))
        assertEquals("ん", kana("n'"))
        assertEquals("ほn", kana("hon"))
        assertEquals("ほんだ", kana("honda"))
    }

    @Test fun pendingTailShownAsRomaji() {
        val c = RomajiComposer()
        assertEquals("k", c.feed("k"))
        assertEquals("ky", c.feed("y"))
        assertEquals("きゃ", c.feed("a"))
    }

    @Test fun reset() {
        val c = RomajiComposer()
        c.feed("nihon")
        c.reset()
        assertEquals("", c.text())
        assertEquals("あ", c.feed("a"))
    }
}
