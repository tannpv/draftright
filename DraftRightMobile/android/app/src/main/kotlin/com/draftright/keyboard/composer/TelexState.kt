package com.draftright.keyboard.composer

object TelexState {
    private val PLAIN_VOWELS = setOf(
        'a', 'e', 'i', 'o', 'u', 'y',
        'A', 'E', 'I', 'O', 'U', 'Y',
    )
    private val SPECIAL_VOWELS = setOf(
        'ă', 'â', 'ê', 'ô', 'ơ', 'ư',
        'Ă', 'Â', 'Ê', 'Ô', 'Ơ', 'Ư',
    )
    private val TONE_MARKS = setOf('s', 'f', 'r', 'x', 'j')

    fun isVowel(c: Char): Boolean = c in PLAIN_VOWELS
    fun isVowelLike(c: Char): Boolean = c in PLAIN_VOWELS || c in SPECIAL_VOWELS
    fun isSpecialVowel(c: Char): Boolean = c in SPECIAL_VOWELS
    fun isToneMark(c: Char): Boolean = c in TONE_MARKS
}
