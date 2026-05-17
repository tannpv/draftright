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

    fun isSpecial(code: Int): Boolean = code < 0
}
