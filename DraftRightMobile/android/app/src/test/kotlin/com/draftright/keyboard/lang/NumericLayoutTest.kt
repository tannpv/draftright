package com.draftright.keyboard.lang

import com.draftright.keyboard.SpecialKeys
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Locks the Samsung-parity numeric keypad (#208): OTP/PIN fields get a
 * number-only pad, NOT the ?123 symbols layer. The layout must expose every
 * digit 0-9 and nothing but digits + the ABC/backspace/enter specials — a
 * stray letter or symbol would defeat the whole point (a clean number pad).
 * TC: NUMPAD-1..3
 */
class NumericLayoutTest {

    private val keys = QwertyLayout.numericRows.flatten()

    @Test fun `all ten digits are present`() {
        val digits = keys.map { it.label }.filter { it.length == 1 && it[0].isDigit() }.toSet()
        assertEquals(setOf("0", "1", "2", "3", "4", "5", "6", "7", "8", "9"), digits)
    }

    @Test fun `every character key is a digit — no letters or symbols`() {
        val charKeys = keys.filter { SpecialKeys.isCharKey(it.code) }
        for (k in charKeys) {
            assertTrue("numeric pad char key '${k.label}' is not a digit", k.label.length == 1 && k.label[0].isDigit())
        }
    }

    @Test fun `only ABC, backspace and enter are allowed as special keys`() {
        val allowed = setOf(SpecialKeys.ALPHA, SpecialKeys.BACKSPACE, SpecialKeys.ENTER)
        val specials = keys.map { it.code }.filter { SpecialKeys.isSpecial(it) }.toSet()
        assertTrue("unexpected special keys on numeric pad: ${specials - allowed}", specials.all { it in allowed })
        // ABC (escape to letters) and backspace must exist so the user is never trapped.
        assertTrue(SpecialKeys.ALPHA in specials)
        assertTrue(SpecialKeys.BACKSPACE in specials)
    }

    @Test fun `no space key on the numeric pad`() {
        assertTrue(keys.none { SpecialKeys.isSpace(it.code) })
    }
}
