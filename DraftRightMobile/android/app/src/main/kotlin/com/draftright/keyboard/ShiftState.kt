package com.draftright.keyboard

/**
 * The three states of the keyboard shift key, mirroring the stock Samsung
 * keyboard: OFF (lowercase), SINGLE (capitalize the next character only, then
 * auto-revert), CAPS_LOCK (capitalize every character until toggled off).
 */
enum class ShiftState { OFF, SINGLE, CAPS_LOCK }

/**
 * Advance the shift state on a single tap, matching the stock Samsung keyboard:
 * one button cycles OFF -> SINGLE -> CAPS_LOCK -> OFF. No double-tap.
 */
fun ShiftState.nextOnTap(): ShiftState = when (this) {
    ShiftState.OFF -> ShiftState.SINGLE
    ShiftState.SINGLE -> ShiftState.CAPS_LOCK
    ShiftState.CAPS_LOCK -> ShiftState.OFF
}

/**
 * Glyph shown on the shift key, giving each state the distinct look of the
 * stock Samsung keyboard:
 *   OFF       ⇧  outline arrow — lowercase
 *   SINGLE    ⬆  filled arrow  — next letter capitalized
 *   CAPS_LOCK ⇪  caps-lock arrow — every letter capitalized
 */
fun ShiftState.glyph(): String = when (this) {
    ShiftState.OFF -> "⇧"
    ShiftState.SINGLE -> "⬆"
    ShiftState.CAPS_LOCK -> "⇪"
}
