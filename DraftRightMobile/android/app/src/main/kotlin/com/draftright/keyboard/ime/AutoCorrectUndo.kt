package com.draftright.keyboard.ime

/**
 * One-shot undo for an auto-correction (#207). Auto-correct is only acceptable
 * if it is trivially reversible, so the correction arms this and the *next*
 * backspace consumes it — putting the typed word back instead of deleting a
 * character.
 *
 * Pure state, no InputConnection/textDocumentProxy: the platform IME owns the
 * text edits, this owns only the decision. Mirror of Swift `AutoCorrectUndo`.
 */
class AutoCorrectUndo {
    private var original: String? = null

    /** The word the correction committed, `null` when nothing is armed. */
    var corrected: String? = null
        private set

    /** Remember that [original] was committed as [corrected]. */
    fun arm(original: String, corrected: String) {
        this.original = original
        this.corrected = corrected
    }

    /**
     * The word to restore, disarming in the same step, or `null` when nothing
     * is armed (so the caller falls through to a normal backspace).
     */
    fun consume(): String? {
        val pending = original
        disarm()
        return pending
    }

    /** Forget the pending correction. */
    fun disarm() {
        original = null
        corrected = null
    }
}
