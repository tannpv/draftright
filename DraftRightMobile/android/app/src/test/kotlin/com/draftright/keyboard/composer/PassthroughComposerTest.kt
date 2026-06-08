package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class PassthroughComposerTest {

    @Test fun `every key passes through`() {
        val c = PassthroughComposer()
        assertTrue(c.onKey('a') is ComposeResult.PassThrough)
        assertTrue(c.onKey(' ') is ComposeResult.PassThrough)
    }

    @Test fun `backspace passes through`() {
        assertTrue(PassthroughComposer().onBackspace() is ComposeResult.PassThrough)
    }

    @Test fun `no composing text`() {
        val c = PassthroughComposer()
        c.onKey('a')
        assertEquals("", c.currentComposingText())
    }
}
