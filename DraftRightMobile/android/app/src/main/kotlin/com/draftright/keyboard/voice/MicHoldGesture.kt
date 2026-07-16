package com.draftright.keyboard.voice

import kotlin.math.hypot

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

    /** True once the finger has slid [slopPx] or more from the down point. */
    fun isCancelArmed(dxPx: Float, dyPx: Float, slopPx: Float): Boolean =
        hypot(dxPx, dyPx) >= slopPx
}
