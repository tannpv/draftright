package com.draftright.keyboard

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Regression coverage for #71: an app (Zalo) clears its text field on "send"
 * without notifying the IME, leaving a stale composer buffer that sticks to the
 * next keystroke ("Tấn" → send → "g" → "Tấng"). [ComposerFieldSync.hasDiverged]
 * is the detection rule; these lock its behaviour.
 */
class ComposerFieldSyncTest {

    // TC: IME71-01 — the bug. Buffer "Tấn" survives a silent field clear.
    @Test
    fun `diverges when field was cleared behind our back`() {
        assertTrue(ComposerFieldSync.hasDiverged(composing = "Tấn", textBeforeCursor = ""))
    }

    // TC: IME71-02 — editor could not report text before cursor.
    @Test
    fun `diverges when editor returns null text before cursor`() {
        assertTrue(ComposerFieldSync.hasDiverged(composing = "Tấn", textBeforeCursor = null))
    }

    // TC: IME71-03 — normal composition: field tail equals the buffer, no reset.
    @Test
    fun `does not diverge during normal composition`() {
        assertFalse(ComposerFieldSync.hasDiverged(composing = "Tấn", textBeforeCursor = "Tấn"))
    }

    // TC: IME71-04 — buffer sits after committed text; tail still matches.
    @Test
    fun `does not diverge when buffer trails committed text`() {
        // getTextBeforeCursor(composing.length) returns only the last N chars,
        // so a committed prefix ("xin ") never reaches this comparison.
        assertFalse(ComposerFieldSync.hasDiverged(composing = "chào", textBeforeCursor = "chào"))
    }

    // TC: IME71-05 — empty buffer is never a divergence (nothing to protect).
    @Test
    fun `never diverges with empty composer`() {
        assertFalse(ComposerFieldSync.hasDiverged(composing = "", textBeforeCursor = ""))
        assertFalse(ComposerFieldSync.hasDiverged(composing = "", textBeforeCursor = null))
        assertFalse(ComposerFieldSync.hasDiverged(composing = "", textBeforeCursor = "anything"))
    }

    // TC: IME71-06 — field replaced by a different word (tapped elsewhere).
    @Test
    fun `diverges when field holds a different word`() {
        assertTrue(ComposerFieldSync.hasDiverged(composing = "Tấn", textBeforeCursor = "xyz"))
    }
}
