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
        assertEquals("ん", kana("nn"))
        assertEquals("ん", kana("n'"))
        assertEquals("ほん", kana("hon"))
        assertEquals("ほんだ", kana("honda"))
    }

    /** A trailing lone "n" finalizes to ん so the kana is dictionary-lookable
     *  ("nihon" → にほん → 日本). Previously it stayed literal ("にほn"), so the
     *  candidate engine never matched and only hiragana showed. */
    @Test fun trailingMoraicNFinalized() {
        assertEquals("にほん", kana("nihon"))
        assertEquals("ほん", kana("hon"))
        assertEquals("ん", kana("n"))
        assertEquals("ん", kana("nn"))
        // mid-word n still binds to a following vowel/consonant (no over-eager ん)
        assertEquals("にほんご", kana("nihongo"))
        assertEquals("こんにちわ", kana("konnichiwa"))
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
