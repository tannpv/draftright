package com.draftright.keyboard.ime

/**
 * Whether the keyboard should open on the digits layer for a field with this
 * `EditorInfo.inputType` (#190) — e.g. OTP, PIN, phone, amount fields.
 *
 * Pure: the bit values are `android.text.InputType` constants (a stable
 * platform ABI), named locally rather than imported so the rule unit-tests on
 * the plain JVM with no `android.jar`, and so the one place that knows "which
 * input classes count as numeric" is here (RULE #1) — the iOS side mirrors it
 * with the equivalent `UIKeyboardType` values.
 */
object NumericField {
    // android.text.InputType — the class mask and the digit-bearing classes.
    private const val TYPE_MASK_CLASS = 0x0000000f
    private const val TYPE_CLASS_NUMBER = 0x00000002
    private const val TYPE_CLASS_PHONE = 0x00000003
    private const val TYPE_CLASS_DATETIME = 0x00000004

    fun isNumeric(inputType: Int): Boolean =
        when (inputType and TYPE_MASK_CLASS) {
            TYPE_CLASS_NUMBER, TYPE_CLASS_PHONE, TYPE_CLASS_DATETIME -> true
            else -> false
        }
}
