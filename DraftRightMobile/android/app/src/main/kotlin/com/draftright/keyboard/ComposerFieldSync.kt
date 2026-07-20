package com.draftright.keyboard

/**
 * Pure decision for whether the composer's in-memory buffer has diverged from
 * the text actually in front of the cursor.
 *
 * Some apps clear their own text field WITHOUT notifying the IME (no
 * onUpdateSelection, no finishComposingText) — Zalo's in-app send button is the
 * canonical case (#71). When that happens the composer still holds the sent word
 * (e.g. "Tấn"), so the next keystroke would compose on top of it → "Tấng".
 *
 * The composing text is always the tail of the text before the cursor, so when
 * the field is untouched `textBeforeCursor` ends with (equals, for a fresh word)
 * the composing buffer. If it no longer does, the field was cleared or changed
 * behind our back and the buffer is stale.
 *
 * Kept pure and free of Android types so the rule is unit-testable and reusable
 * across any keystroke path.
 */
object ComposerFieldSync {
    /**
     * @param composing the composer's current composing text (may be empty)
     * @param textBeforeCursor the last [composing].length chars before the
     *   cursor as reported by the editor (null when the editor can't provide it)
     * @return true when the buffer is non-empty and no longer matches the field,
     *   meaning the caller should drop the stale buffer before the next keystroke
     */
    fun hasDiverged(composing: String, textBeforeCursor: String?): Boolean {
        if (composing.isEmpty()) return false
        return textBeforeCursor != composing
    }
}
