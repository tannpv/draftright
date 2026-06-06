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
 * Name of the Material shift icon for each state, giving the three states the
 * distinct look of the stock Samsung keyboard:
 *   OFF       outline shift arrow — lowercase
 *   SINGLE    filled shift arrow  — next letter capitalized
 *   CAPS_LOCK shift-lock arrow     — every letter capitalized
 *
 * Resolved to a drawable by [KeyIcons]; kept as a name here so the mapping is
 * unit-testable without pulling in Android resources.
 */
fun ShiftState.iconName(): String = when (this) {
    ShiftState.OFF -> "shift"
    ShiftState.SINGLE -> "shift_fill"
    ShiftState.CAPS_LOCK -> "shift_lock"
}
