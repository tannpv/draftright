package com.draftright.keyboard

import android.media.AudioManager
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Locks the Samsung-parity key-feedback mapping: every key actuation maps to a
 * key-kind, and each kind to the platform sound effect Samsung/Gboard use
 * (standard click for letters, distinct spacebar / delete / return clicks).
 *
 * These are the PURE halves of [KeyFeedback] — the actual haptic + sound firing
 * touches Android (View.performHapticFeedback / AudioManager.playSoundEffect,
 * both already gated on the OS haptic / touch-sound settings) and is verified
 * on-device, not here. Testing the mapping guards the one chokepoint every key
 * routes through: a new key code can never fall through without a sound.
 * TC: KBD-FEEDBACK-1..4
 */
class KeyFeedbackTest {

    @Test fun `letter key uses the standard keypress sound`() {
        assertEquals(KeyFeedbackKind.CHAR, KeyFeedback.kindOf('a'.code))
        assertEquals(AudioManager.FX_KEYPRESS_STANDARD, KeyFeedback.soundEffect(KeyFeedbackKind.CHAR))
    }

    @Test fun `space uses the spacebar sound`() {
        assertEquals(KeyFeedbackKind.SPACE, KeyFeedback.kindOf(SpecialKeys.SPACE_CODE))
        assertEquals(AudioManager.FX_KEYPRESS_SPACEBAR, KeyFeedback.soundEffect(KeyFeedbackKind.SPACE))
    }

    @Test fun `backspace uses the delete sound`() {
        assertEquals(KeyFeedbackKind.DELETE, KeyFeedback.kindOf(SpecialKeys.BACKSPACE))
        assertEquals(AudioManager.FX_KEYPRESS_DELETE, KeyFeedback.soundEffect(KeyFeedbackKind.DELETE))
    }

    @Test fun `enter uses the return sound`() {
        assertEquals(KeyFeedbackKind.ENTER, KeyFeedback.kindOf(SpecialKeys.ENTER))
        assertEquals(AudioManager.FX_KEYPRESS_RETURN, KeyFeedback.soundEffect(KeyFeedbackKind.ENTER))
    }

    // Functional keys that aren't delete/enter (shift, layer switch, globe) still
    // get feedback — the standard click, matching Samsung. Nothing falls silent.
    @Test fun `other special keys fall back to the standard sound`() {
        for (code in intArrayOf(
            SpecialKeys.SHIFT, SpecialKeys.SYMBOLS, SpecialKeys.SYMBOLS2,
            SpecialKeys.ALPHA, SpecialKeys.GLOBE, SpecialKeys.GLOBE_PICKER,
        )) {
            assertEquals(KeyFeedbackKind.OTHER, KeyFeedback.kindOf(code))
        }
        assertEquals(AudioManager.FX_KEYPRESS_STANDARD, KeyFeedback.soundEffect(KeyFeedbackKind.OTHER))
    }

    // soundEffect must be total over the enum — a new kind can't compile without
    // a sound (guards the chokepoint at the type level).
    @Test fun `every kind maps to a sound`() {
        for (kind in KeyFeedbackKind.values()) {
            KeyFeedback.soundEffect(kind) // must not throw / must be exhaustive
        }
    }
}
