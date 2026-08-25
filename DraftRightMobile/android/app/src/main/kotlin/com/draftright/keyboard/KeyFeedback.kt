package com.draftright.keyboard

import android.content.Context
import android.media.AudioManager
import android.view.HapticFeedbackConstants
import android.view.View

/** The feedback-relevant class of a key press — decides which click plays. */
enum class KeyFeedbackKind { CHAR, SPACE, DELETE, ENTER, OTHER }

/**
 * Samsung-parity key feedback: a short haptic tick + a key-appropriate click on
 * every key actuation. Both effects go through the platform APIs that already
 * respect the user's OS settings — [View.performHapticFeedback] honours the
 * system haptic toggle, and [AudioManager.playSoundEffect] is silent when
 * "touch sounds" is off — so there is no separate settings gate to maintain.
 *
 * This is the single chokepoint every key routes through (Rule #1: feedback is
 * cross-cutting, so it lives in one place invoked from the touch handler, not
 * copied per key). The kind→sound mapping is pure and unit-tested
 * ([KeyFeedbackTest]); the firing itself is verified on-device.
 *
 * @param host any attached keyboard view — used as the haptic source; a single
 *             host works for every key since haptics aren't spatial.
 */
class KeyFeedback(private val host: View) {

    // Nullable: a handful of OEM/emulator builds expose no AUDIO_SERVICE. Missing
    // audio must degrade to "haptic only", never crash the keystroke.
    private val audio: AudioManager? =
        host.context.getSystemService(Context.AUDIO_SERVICE) as? AudioManager

    /** Fire haptic + sound for the key with the given [code]. Safe to call on every press. */
    fun onKey(code: Int) {
        // KEYBOARD_TAP is the constant OS keyboards use; it obeys the system
        // haptic setting without us reading it.
        host.performHapticFeedback(HapticFeedbackConstants.KEYBOARD_TAP)
        audio?.playSoundEffect(soundEffect(kindOf(code)))
    }

    companion object {
        /** Classify a key [code] (as used by [SpecialKeys]) into a feedback kind. */
        fun kindOf(code: Int): KeyFeedbackKind = when {
            SpecialKeys.isSpace(code) -> KeyFeedbackKind.SPACE
            code == SpecialKeys.BACKSPACE -> KeyFeedbackKind.DELETE
            code == SpecialKeys.ENTER -> KeyFeedbackKind.ENTER
            SpecialKeys.isCharKey(code) -> KeyFeedbackKind.CHAR
            else -> KeyFeedbackKind.OTHER // shift / layer switch / globe
        }

        /**
         * Platform sound-effect id for a kind. Matches Samsung/Gboard: letters
         * click "standard", space / delete / return have their own tones.
         */
        fun soundEffect(kind: KeyFeedbackKind): Int = when (kind) {
            KeyFeedbackKind.SPACE -> AudioManager.FX_KEYPRESS_SPACEBAR
            KeyFeedbackKind.DELETE -> AudioManager.FX_KEYPRESS_DELETE
            KeyFeedbackKind.ENTER -> AudioManager.FX_KEYPRESS_RETURN
            KeyFeedbackKind.CHAR, KeyFeedbackKind.OTHER -> AudioManager.FX_KEYPRESS_STANDARD
        }
    }
}
