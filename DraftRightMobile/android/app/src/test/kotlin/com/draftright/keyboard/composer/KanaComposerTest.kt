package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Flick kana composer (#212): buffers kana as-is for the kana→kanji engine.
 * TC: JPFLICK-6
 */
class KanaComposerTest {

    private fun type(vararg kana: Char): String {
        val c = KanaComposer()
        var last: ComposeResult = ComposeResult.PassThrough
        for (k in kana) last = c.onKey(k)
        return when (last) {
            is ComposeResult.Composing -> last.text
            is ComposeResult.Commit -> last.text
            else -> c.currentComposingText()
        }
    }

    @Test fun `buffers kana as the composing text`() {
        assertEquals("にほんご", type('に', 'ほ', 'ん', 'ご'))
        assertEquals("かんじ", type('か', 'ん', 'じ'))
    }

    @Test fun `accepts choonpu and wave dash`() {
        assertEquals("らーめん", type('ら', 'ー', 'め', 'ん'))
    }

    @Test fun `backspace strips the last kana`() {
        val c = KanaComposer()
        c.onKey('か'); c.onKey('な')
        c.onBackspace()
        assertEquals("か", c.currentComposingText())
    }

    @Test fun `non-input char commits the buffer`() {
        val c = KanaComposer()
        c.onKey('か')
        val r = c.onKey(' ') // space isn't an input char here
        assertTrue(r is ComposeResult.Commit)
    }
}
