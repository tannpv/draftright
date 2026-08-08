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

    /**
     * Vowels that can sit AFTER the nucleus inside one cluster without being a
     * nucleus themselves. Lives here with the other vowel classifications so
     * there is one place that answers "what kind of vowel is this" (#152).
     */
    private val GLIDE_CODAS = setOf('i', 'y', 'u', 'o', 'I', 'Y', 'U', 'O')

    fun isVowel(c: Char): Boolean = c in PLAIN_VOWELS
    fun isVowelLike(c: Char): Boolean = c in PLAIN_VOWELS || c in SPECIAL_VOWELS
    fun isSpecialVowel(c: Char): Boolean = c in SPECIAL_VOWELS
    fun isToneMark(c: Char): Boolean = c in TONE_MARKS

    /**
     * True for an offglide — a vowel a trailing modifier may reach back over
     * to find its nucleus ("day" + a: the 'y' is an offglide, the 'a' is the
     * nucleus and takes the circumflex). Reaching past a non-glide vowel would
     * corrupt genuine clusters like "oeo" (ngoẻo).
     */
    fun isGlideCoda(c: Char): Boolean = c in GLIDE_CODAS
}
