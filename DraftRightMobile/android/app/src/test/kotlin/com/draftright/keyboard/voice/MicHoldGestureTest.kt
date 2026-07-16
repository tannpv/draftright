package com.draftright.keyboard.voice

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class MicHoldGestureTest {
    private val slop = 96f

    @Test fun `no movement is not cancel-armed`() {
        assertFalse(MicHoldGesture.isCancelArmed(0f, 0f, slop))
    }

    @Test fun `small jitter within slop is not cancel-armed`() {
        assertFalse(MicHoldGesture.isCancelArmed(20f, -20f, slop))
    }

    @Test fun `sliding up past slop is cancel-armed`() {
        assertTrue(MicHoldGesture.isCancelArmed(0f, -200f, slop))
    }

    @Test fun `sliding left past slop is cancel-armed`() {
        assertTrue(MicHoldGesture.isCancelArmed(-200f, 0f, slop))
    }

    @Test fun `sliding down within field is not cancel-armed at small distance`() {
        assertFalse(MicHoldGesture.isCancelArmed(0f, 40f, slop))
    }

    @Test fun `sliding down far is NOT cancel-armed`() {
        assertFalse(MicHoldGesture.isCancelArmed(0f, 300f, slop))
    }

    @Test fun `sliding right far is NOT cancel-armed`() {
        assertFalse(MicHoldGesture.isCancelArmed(300f, 0f, slop))
    }
}
