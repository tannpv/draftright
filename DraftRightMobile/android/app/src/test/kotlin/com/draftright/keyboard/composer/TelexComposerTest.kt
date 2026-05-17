package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test

class TelexComposerTest {

    private fun finalText(c: TelexComposer, vararg chars: Char): String {
        var last: ComposeResult = ComposeResult.PassThrough
        for (ch in chars) last = c.onKey(ch)
        return when (last) {
            is ComposeResult.Composing -> last.text
            is ComposeResult.Commit -> last.text
            else -> c.currentComposingText()
        }
    }

    @Test fun `aa composes â`() { assertEquals("â", finalText(TelexComposer(), 'a', 'a')) }
    @Test fun `oo composes ô`() { assertEquals("ô", finalText(TelexComposer(), 'o', 'o')) }
    @Test fun `ee composes ê`() { assertEquals("ê", finalText(TelexComposer(), 'e', 'e')) }
    @Test fun `ow composes ơ`() { assertEquals("ơ", finalText(TelexComposer(), 'o', 'w')) }
    @Test fun `uw composes ư`() { assertEquals("ư", finalText(TelexComposer(), 'u', 'w')) }
    @Test fun `aw composes ă`() { assertEquals("ă", finalText(TelexComposer(), 'a', 'w')) }
    @Test fun `dd composes đ`() { assertEquals("đ", finalText(TelexComposer(), 'd', 'd')) }
    @Test fun `aaj composes ậ`() { assertEquals("ậ", finalText(TelexComposer(), 'a', 'a', 'j')) }
    @Test fun `uow composes ươ`() { assertEquals("ươ", finalText(TelexComposer(), 'u', 'o', 'w')) }
    @Test fun `uowj composes ượ`() { assertEquals("ượ", finalText(TelexComposer(), 'u', 'o', 'w', 'j')) }
    @Test fun `AA composes Â`() { assertEquals("Â", finalText(TelexComposer(), 'A', 'A')) }

    @Test
    fun `q is direct-commit (no rule)`() {
        val c = TelexComposer()
        val r = c.onKey('q')
        assertTrue(r is ComposeResult.Composing)
    }

    @Test fun `as composes á`() { assertEquals("á", finalText(TelexComposer(), 'a', 's')) }
    @Test fun `af composes à`() { assertEquals("à", finalText(TelexComposer(), 'a', 'f')) }
    @Test fun `ar composes ả`() { assertEquals("ả", finalText(TelexComposer(), 'a', 'r')) }
    @Test fun `ax composes ã`() { assertEquals("ã", finalText(TelexComposer(), 'a', 'x')) }
    @Test fun `aj composes ạ`() { assertEquals("ạ", finalText(TelexComposer(), 'a', 'j')) }

    @Test
    fun `vietj composes việt`() {
        assertEquals("việt", finalText(TelexComposer(), 'v', 'i', 'e', 't', 'j'))
    }

    @Test
    fun `chuong sequence composes chương`() {
        assertEquals("chương", finalText(TelexComposer(), 'c', 'h', 'u', 'o', 'w', 'n', 'g'))
    }

    @Test
    fun `nguoiwf composes người`() {
        assertEquals("người", finalText(TelexComposer(), 'n', 'g', 'u', 'o', 'w', 'i', 'f'))
    }

    @Test
    fun `tiengs composes tiếng`() {
        assertEquals("tiếng", finalText(TelexComposer(), 't', 'i', 'e', 's', 'n', 'g'))
    }

    @Test
    fun `backspace from việt yields việ`() {
        val c = TelexComposer()
        finalText(c, 'v', 'i', 'e', 't', 'j')
        val r = c.onBackspace()
        assertTrue(r is ComposeResult.Composing)
        assertEquals("việ", (r as ComposeResult.Composing).text)
    }

    @Test
    fun `backspace empty composer passes through`() {
        val r = TelexComposer().onBackspace()
        assertTrue(r is ComposeResult.PassThrough)
    }

    @Test
    fun `backspace from â yields a`() {
        val c = TelexComposer()
        finalText(c, 'a', 'a')
        val r = c.onBackspace()
        assertTrue(r is ComposeResult.Composing)
        assertEquals("a", (r as ComposeResult.Composing).text)
    }

    @Test
    fun `backspace from ậ yields â`() {
        val c = TelexComposer()
        finalText(c, 'a', 'a', 'j')
        val r = c.onBackspace()
        assertTrue(r is ComposeResult.Composing)
        assertEquals("â", (r as ComposeResult.Composing).text)
    }

    @Test
    fun `multiple backspaces fully clear viet composing state`() {
        val c = TelexComposer()
        finalText(c, 'v', 'i', 'e', 't', 'j')
        var safety = 0
        while (c.currentComposingText().isNotEmpty() && safety++ < 20) c.onBackspace()
        assertEquals("", c.currentComposingText())
        val again = c.onBackspace()
        assertTrue(again is ComposeResult.PassThrough)
    }

    @Test
    fun `space mid-cluster commits pending`() {
        val c = TelexComposer()
        c.onKey('v'); c.onKey('i'); c.onKey('e')
        val r = c.onKey(' ')
        assertTrue(r is ComposeResult.Commit)
        assertEquals("vie ", (r as ComposeResult.Commit).text)
    }

    @Test
    fun `reset clears state`() {
        val c = TelexComposer()
        c.onKey('a'); c.onKey('a')
        c.reset()
        assertEquals("", c.currentComposingText())
    }

    @Test
    fun `length cap commits at 32 chars`() {
        val c = TelexComposer()
        var lastCommit: ComposeResult? = null
        repeat(33) {
            val r = c.onKey('q')
            if (r is ComposeResult.Commit) lastCommit = r
        }
        assertNotNull("expected commit at length cap", lastCommit)
    }
}
