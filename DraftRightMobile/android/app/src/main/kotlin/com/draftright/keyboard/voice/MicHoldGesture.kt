package com.draftright.keyboard.voice

/**
 * Pure geometry for the hold-to-talk mic gesture. Kept android-free so it is
 * unit-testable without touch events. The view feeds displacement from the
 * touch-down point; this decides whether the finger has slid far enough to
 * mean "cancel" (slide-away-to-cancel, Zalo/WhatsApp style).
 */
object MicHoldGesture {
    /** Delay before a hold starts recording; a shorter press is a no-op tap. */
    const val ARM_MS = 180L
    /** Default slide-to-cancel threshold, in dp (the view converts to px). */
    const val DEFAULT_SLOP_DP = 32

    /**
     * True once the finger has slid [slopPx] or more up or left from the down
     * point. Screen y grows downward, so "up" is a negative dy and "left" is
     * a negative dx. Deliberately NOT omnidirectional: a finger relaxing or
     * the phone tilting down while speaking is common and shouldn't silently
     * discard a good dictation, so downward/rightward drift never arms
     * cancel.
     */
    fun isCancelArmed(dxPx: Float, dyPx: Float, slopPx: Float): Boolean =
        dyPx <= -slopPx || dxPx <= -slopPx
}
