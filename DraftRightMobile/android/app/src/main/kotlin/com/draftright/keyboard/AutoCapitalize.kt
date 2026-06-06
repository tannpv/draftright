package com.draftright.keyboard

/**
 * Decides the auto-capitalization shift state, matching the stock Samsung /
 * Gboard behaviour: the shift key auto-engages "capitalize-next" (SINGLE) at
 * the start of a field and after sentence-ending punctuation, and reverts to
 * OFF elsewhere — but never overrides a user-engaged CAPS_LOCK.
 *
 * Kept as a pure function so the policy is unit-testable without an Android
 * InputConnection: the IME supplies [capsMode] from
 * `InputConnection.getCursorCapsMode(editorInfo.inputType)`, which already
 * folds in the field's caps flags and the cursor position.
 */
object AutoCapitalize {
    /**
     * @param current  the current shift state, so a manual CAPS_LOCK survives.
     * @param capsMode the value from getCursorCapsMode(); nonzero means the
     *                 platform wants the next character capitalized.
     */
    fun resolve(current: ShiftState, capsMode: Int): ShiftState = when {
        current == ShiftState.CAPS_LOCK -> ShiftState.CAPS_LOCK
        capsMode != 0 -> ShiftState.SINGLE
        else -> ShiftState.OFF
    }
}
