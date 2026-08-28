package com.draftright.keyboard.ime

/**
 * Hiragana → katakana transliteration (#211/#212): every JP IME offers the
 * katakana form of the typed reading as a candidate (かな → カナ), which is how
 * users write loanwords (コーヒー, パソコン) and names. The hiragana block
 * U+3041..U+3096 maps 1:1 to katakana U+30A1..U+30F6 by a fixed +0x60 offset
 * (Unicode standard); ー (chōonpu), 〜 and anything outside the block pass
 * through unchanged. Pure function, mirrored in Swift `Katakana`.
 */
object Katakana {
    private const val HIRA_START = 0x3041
    private const val HIRA_END = 0x3096
    private const val TO_KATAKANA = 0x60

    fun fromHiragana(reading: String): String = buildString(reading.length) {
        for (ch in reading) {
            val code = ch.code
            append(if (code in HIRA_START..HIRA_END) (code + TO_KATAKANA).toChar() else ch)
        }
    }
}
