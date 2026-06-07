package com.draftright.keyboard

object SpecialKeys {
    const val SHIFT = -1
    const val SYMBOLS = -2
    const val GLOBE = -3
    const val ENTER = -4
    const val BACKSPACE = -5
    const val SYMBOLS2 = -6
    const val ALPHA = -7
    const val GLOBE_PICKER = -8

    val SPACE_CODE: Int = ' '.code

    // How long the user must hold a key before the long-press accent
    // picker fires.
    const val LONG_PRESS_MS = 300L

    // Two shift taps within this window toggle CAPS LOCK.
    const val DOUBLE_TAP_MS = 300L

    // Horizontal travel on the space bar (px) that triggers a language
    // cycle. ~80 dp at 420 dpi ≈ 168 px. Tuned so a deliberate horizontal
    // drag fires but a tap or vertical jitter does not.
    const val SPACE_SWIPE_THRESHOLD_PX = 80f

    // Keys whose press-feedback popup is suppressed (their narrow glyphs
    // look poor when blown up in the floating preview).
    val NO_PREVIEW_LABELS: Set<String> = setOf(",", ".")

    fun isSpecial(code: Int): Boolean = code < 0
    fun isSpace(code: Int): Boolean = code == SPACE_CODE
    fun isCharKey(code: Int): Boolean = code >= 0 && code != SPACE_CODE
}
