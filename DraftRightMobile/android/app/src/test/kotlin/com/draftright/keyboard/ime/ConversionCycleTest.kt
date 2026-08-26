package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * The JP/ZH space-cycle cursor (#207). TC: CJKCYC-1..5
 */
class ConversionCycleTest {

    @Test fun `idle by default`() {
        val c = ConversionCycle()
        assertFalse(c.isActive)
        assertNull(c.current())
        assertNull(c.advance())
    }

    @Test fun `start selects the top candidate`() {
        val c = ConversionCycle()
        c.start(listOf("漢字", "幹事", "感じ"))
        assertTrue(c.isActive)
        assertEquals("漢字", c.current())
    }

    @Test fun `advance walks the list then wraps`() {
        val c = ConversionCycle()
        c.start(listOf("漢字", "幹事", "感じ"))
        assertEquals("幹事", c.advance())
        assertEquals("感じ", c.advance())
        assertEquals("漢字", c.advance()) // wraps back to top
    }

    @Test fun `reset returns to idle`() {
        val c = ConversionCycle()
        c.start(listOf("私"))
        c.reset()
        assertFalse(c.isActive)
        assertNull(c.current())
    }

    @Test fun `start with empty list stays idle`() {
        val c = ConversionCycle()
        c.start(emptyList())
        assertFalse(c.isActive)
        assertNull(c.current())
    }

    @Test fun `single candidate advance stays on it`() {
        val c = ConversionCycle()
        c.start(listOf("私"))
        assertEquals("私", c.advance())
        assertEquals("私", c.advance())
    }
}
