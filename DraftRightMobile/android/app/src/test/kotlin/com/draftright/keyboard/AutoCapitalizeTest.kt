package com.draftright.keyboard

import android.text.TextUtils
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Locks the Samsung/Gboard auto-capitalization behaviour: the shift key
 * auto-engages "capitalize-next" (SINGLE) at the start of a field and after
 * sentence-ending punctuation, driven off the platform's getCursorCapsMode()
 * result — without ever overriding a user-engaged CAPS_LOCK.
 */
class AutoCapitalizeTest {

    // getCursorCapsMode() returns a nonzero flag when the platform wants the
    // next character capitalized (field start / after . ! ?). Use the real
    // sentence flag so the contract matches what the IME passes in.
    private val capsActive = TextUtils.CAP_MODE_SENTENCES
    private val capsInactive = 0

    @Test fun `field start with sentence caps engages single shift`() {
        assertEquals(ShiftState.SINGLE, AutoCapitalize.resolve(ShiftState.OFF, capsActive))
    }

    @Test fun `mid word with no caps mode stays off`() {
        assertEquals(ShiftState.OFF, AutoCapitalize.resolve(ShiftState.OFF, capsInactive))
    }

    @Test fun `caps lock is preserved even when caps mode inactive`() {
        assertEquals(ShiftState.CAPS_LOCK, AutoCapitalize.resolve(ShiftState.CAPS_LOCK, capsInactive))
    }

    @Test fun `caps lock is preserved when caps mode active`() {
        assertEquals(ShiftState.CAPS_LOCK, AutoCapitalize.resolve(ShiftState.CAPS_LOCK, capsActive))
    }

    @Test fun `single shift recomputes to off when caps mode goes inactive`() {
        // A leftover one-shot SINGLE (e.g. user moved the cursor mid-word) is
        // re-evaluated, matching Samsung re-reading caps state on cursor move.
        assertEquals(ShiftState.OFF, AutoCapitalize.resolve(ShiftState.SINGLE, capsInactive))
    }

    @Test fun `single shift stays single when caps mode still active`() {
        assertEquals(ShiftState.SINGLE, AutoCapitalize.resolve(ShiftState.SINGLE, capsActive))
    }
}
