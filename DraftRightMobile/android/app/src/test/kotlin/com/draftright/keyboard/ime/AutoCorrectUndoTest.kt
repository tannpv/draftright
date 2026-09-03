package com.draftright.keyboard.ime

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * One-shot undo armed by an auto-correction (#207): the backspace immediately
 * after a correction puts the typed word back instead of deleting a character.
 * Mirror of Swift `AutoCorrectUndoTests`.
 */
class AutoCorrectUndoTest {

    @Test
    fun backspaceAfterCorrectionRevertsOnce() {
        val undo = AutoCorrectUndo()
        undo.arm(original = "khôg", corrected = "không")
        assertEquals("không", undo.corrected)
        assertEquals("khôg", undo.consume())
        assertNull("undo is one-shot", undo.consume())
    }

    @Test
    fun nothingToConsumeBeforeAnyCorrection() {
        assertNull(AutoCorrectUndo().consume())
    }

    @Test
    fun disarmDropsThePendingUndo() {
        val undo = AutoCorrectUndo()
        undo.arm(original = "khôg", corrected = "không")
        undo.disarm()
        assertNull(undo.consume())
    }

    @Test
    fun armingAgainReplacesThePendingUndo() {
        val undo = AutoCorrectUndo()
        undo.arm(original = "khôg", corrected = "không")
        undo.arm(original = "anb", corrected = "anh")
        assertEquals("anb", undo.consume())
    }

    @Test
    fun isLiveOnlyWhileFieldEndsWithCorrectionPlusSpace() {
        val undo = AutoCorrectUndo()
        assertFalse("not armed", undo.isLive("không "))
        undo.arm(original = "khôg", corrected = "không")
        assertTrue(undo.isLive("không "))
        assertTrue("suffix match is enough", undo.isLive("chào không "))
        assertFalse("space not yet appended", undo.isLive("không"))
        assertFalse("user typed past it", undo.isLive("không x"))
        assertFalse("field unreadable", undo.isLive(null))
    }
}
