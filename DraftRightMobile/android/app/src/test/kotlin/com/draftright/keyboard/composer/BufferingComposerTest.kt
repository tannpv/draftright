package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/** A trivial subclass (transform = uppercase) to test the shared skeleton. */
private class UpperComposer : BufferingComposer() {
    override fun transform(raw: String): String = raw.uppercase()
}

class BufferingComposerTest {

    @Test fun `accumulates and transforms`() {
        val c = UpperComposer()
        c.onKey('a'); c.onKey('b')
        assertEquals("AB", c.currentComposingText())
    }

    @Test fun `letter returns Composing with transformed text`() {
        val r = UpperComposer().onKey('a')
        assertTrue(r is ComposeResult.Composing)
        assertEquals("A", (r as ComposeResult.Composing).text)
    }

    @Test fun `non-letter commits transformed buffer plus char`() {
        val c = UpperComposer()
        c.onKey('a'); c.onKey('b')
        val r = c.onKey(' ')
        assertEquals("AB ", (r as ComposeResult.Commit).text)
        assertEquals("", c.currentComposingText())
    }

    @Test fun `non-letter on empty passes through`() {
        assertTrue(UpperComposer().onKey(' ') is ComposeResult.PassThrough)
    }

    @Test fun `backspace strips then consumes when empty`() {
        val c = UpperComposer()
        c.onKey('a')
        assertTrue(c.onBackspace() is ComposeResult.Consumed)
        assertEquals("", c.currentComposingText())
    }

    @Test fun `reset clears buffer`() {
        val c = UpperComposer()
        c.onKey('a'); c.reset()
        assertEquals("", c.currentComposingText())
    }
}
