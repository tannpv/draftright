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
    fun `nguyeexn composes nguyễn`() {
        // uyê cluster — tilde must land on ê, not y.
        assertEquals("nguyễn", finalText(TelexComposer(), 'n', 'g', 'u', 'y', 'e', 'e', 'x', 'n'))
    }

    @Test
    fun `nguyeenx composes nguyễn`() {
        // Same word, tilde typed after the trailing consonant.
        assertEquals("nguyễn", finalText(TelexComposer(), 'n', 'g', 'u', 'y', 'e', 'e', 'n', 'x'))
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

    @Test fun `ass cancels tone to as`() { assertEquals("as", finalText(TelexComposer(), 'a', 's', 's')) }
    @Test fun `aff cancels tone to af`() { assertEquals("af", finalText(TelexComposer(), 'a', 'f', 'f')) }
    @Test fun `arr cancels tone to ar`() { assertEquals("ar", finalText(TelexComposer(), 'a', 'r', 'r')) }
    @Test fun `axx cancels tone to ax`() { assertEquals("ax", finalText(TelexComposer(), 'a', 'x', 'x')) }
    @Test fun `ajj cancels tone to aj`() { assertEquals("aj", finalText(TelexComposer(), 'a', 'j', 'j')) }
    @Test fun `vietjj cancels tone only keeping ê mark`() { assertEquals("viêtj", finalText(TelexComposer(), 'v', 'i', 'e', 't', 'j', 'j')) }
    @Test fun `aaa cancels circumflex to aaa`() { assertEquals("aa", finalText(TelexComposer(), 'a', 'a', 'a')) }
    @Test fun `ooo cancels circumflex to oo`() { assertEquals("oo", finalText(TelexComposer(), 'o', 'o', 'o')) }
    @Test fun `eee cancels circumflex to ee`() { assertEquals("ee", finalText(TelexComposer(), 'e', 'e', 'e')) }
    @Test fun `aww cancels breve to aw`() { assertEquals("aw", finalText(TelexComposer(), 'a', 'w', 'w')) }
    @Test fun `oww cancels horn to ow`() { assertEquals("ow", finalText(TelexComposer(), 'o', 'w', 'w')) }
    @Test fun `uww cancels horn to uw`() { assertEquals("uw", finalText(TelexComposer(), 'u', 'w', 'w')) }
    @Test fun `uoww cancels uow cluster to uow`() { assertEquals("uow", finalText(TelexComposer(), 'u', 'o', 'w', 'w')) }
    @Test fun `ddd cancels d to dd`() { assertEquals("dd", finalText(TelexComposer(), 'd', 'd', 'd')) }
    @Test fun `as then s cancel preserves case ASS yields AS`() { assertEquals("AS", finalText(TelexComposer(), 'A', 'S', 'S')) }

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
