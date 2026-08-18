package com.draftright.keyboard.ime

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** #190 — the digits-layer decision for a field's EditorInfo.inputType. */
class NumericFieldTest {

    // android.text.InputType values, restated so the test asserts against the
    // real platform constants (not the copy inside NumericField).
    private val TYPE_CLASS_TEXT = 0x00000001
    private val TYPE_CLASS_NUMBER = 0x00000002
    private val TYPE_CLASS_PHONE = 0x00000003
    private val TYPE_CLASS_DATETIME = 0x00000004
    private val TYPE_NUMBER_VARIATION_PASSWORD = 0x00000010 // an upper-bit flag
    private val TYPE_TEXT_FLAG_MULTI_LINE = 0x00020000

    @Test
    fun numericClassesUseDigitsLayer() {
        assertTrue(NumericField.isNumeric(TYPE_CLASS_NUMBER))
        assertTrue(NumericField.isNumeric(TYPE_CLASS_PHONE))
        assertTrue(NumericField.isNumeric(TYPE_CLASS_DATETIME))
    }

    @Test
    fun numericClassWithUpperFlagsStillNumeric() {
        // Real fields OR variation/flag bits above the class nibble; the mask
        // must ignore them (a numeric-password field is still digits).
        assertTrue(NumericField.isNumeric(TYPE_CLASS_NUMBER or TYPE_NUMBER_VARIATION_PASSWORD))
    }

    @Test
    fun textAndUnspecifiedUseAlpha() {
        assertFalse(NumericField.isNumeric(TYPE_CLASS_TEXT))
        assertFalse(NumericField.isNumeric(TYPE_CLASS_TEXT or TYPE_TEXT_FLAG_MULTI_LINE))
        assertFalse(NumericField.isNumeric(0)) // TYPE_NULL — no class set
    }
}
