package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class RomajiKanaComposerTest {

    private fun typeAll(c: RomajiKanaComposer, s: String): ComposeResult {
        var r: ComposeResult = ComposeResult.PassThrough
        for (ch in s) r = c.onKey(ch)
        return r
    }

    @Test fun `watashi composes わたし`() {
        val c = RomajiKanaComposer()
        typeAll(c, "watashi")
        assertEquals("わたし", c.currentComposingText())
    }

    @Test fun `nihongo composes にほんご`() {
        val c = RomajiKanaComposer()
        typeAll(c, "nihongo")
        assertEquals("にほんご", c.currentComposingText())
    }

    @Test fun `non-letter commits kana plus char`() {
        val c = RomajiKanaComposer()
        typeAll(c, "watashi")
        val r = c.onKey(' ')
        assertTrue(r is ComposeResult.Commit)
        assertEquals("わたし ", (r as ComposeResult.Commit).text)
        assertEquals("", c.currentComposingText())
    }

    @Test fun `backspace strips one romaji char`() {
        val c = RomajiKanaComposer()
        typeAll(c, "wa")        // わ
        assertEquals("わ", c.currentComposingText())
        c.onBackspace()         // w
        assertEquals("w", c.currentComposingText())
    }

    @Test fun `non-letter with empty buffer passes through`() {
        val c = RomajiKanaComposer()
        assertTrue(c.onKey(' ') is ComposeResult.PassThrough)
    }
}
