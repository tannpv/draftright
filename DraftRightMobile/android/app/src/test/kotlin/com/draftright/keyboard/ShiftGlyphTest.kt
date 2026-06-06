package com.draftright.keyboard

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * The three shift states must render as three visually distinct glyphs, so a
 * user can tell "cap first" (SINGLE) from "normal" (OFF) at a glance — the
 * stock Samsung keyboard shows a different icon for each.
 */
class ShiftGlyphTest {

    @Test fun `each state has a non-blank glyph`() {
        ShiftState.values().forEach { assertTrue(it.name, it.glyph().isNotBlank()) }
    }

    @Test fun `all three glyphs are distinct`() {
        val glyphs = ShiftState.values().map { it.glyph() }
        assertEquals(glyphs.size, glyphs.toSet().size)
    }
}
