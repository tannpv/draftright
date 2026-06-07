package com.draftright.keyboard

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Locks the single-tap shift cycle to match the stock Samsung keyboard:
 * each tap advances OFF -> SINGLE (cap first) -> CAPS_LOCK (cap all) -> OFF.
 * No double-tap; one button cycles all three states.
 */
class ShiftCycleTest {

    @Test fun `off advances to single`() {
        assertEquals(ShiftState.SINGLE, ShiftState.OFF.nextOnTap())
    }

    @Test fun `single advances to caps lock`() {
        assertEquals(ShiftState.CAPS_LOCK, ShiftState.SINGLE.nextOnTap())
    }

    @Test fun `caps lock wraps back to off`() {
        assertEquals(ShiftState.OFF, ShiftState.CAPS_LOCK.nextOnTap())
    }

    @Test fun `three taps return to the starting state`() {
        assertEquals(
            ShiftState.OFF,
            ShiftState.OFF.nextOnTap().nextOnTap().nextOnTap(),
        )
    }
}
