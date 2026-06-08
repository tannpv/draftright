package com.draftright.keyboard.composer

/**
 * Assembles a sequence of Hangul compatibility jamo (ㄱ, ㅏ, ㄴ, …) into Hangul
 * syllable blocks (가, 한, 녕, …). This is the whole of Korean input — deterministic
 * composition, no dictionary / candidate selection (unlike JP/ZH).
 *
 * Pure function: given the full jamo string it returns the assembled text, so it
 * fits BufferingComposer.transform (recomputed + memoized per keystroke).
 *
 * Handles: initial(cho)+medial(jung)+final(jong) blocks, compound vowels
 * (ㅗ+ㅏ=ㅘ), compound finals (ㄱ+ㅅ=ㄳ), and the final→initial "move" when a
 * vowel follows a complete CVC block (간 + ㅣ → 가 + 니).
 */
object HangulAssembler {

    private const val CHO = "ㄱㄲㄴㄷㄸㄹㅁㅂㅃㅅㅆㅇㅈㅉㅊㅋㅌㅍㅎ"
    private const val JUNG = "ㅏㅐㅑㅒㅓㅔㅕㅖㅗㅘㅙㅚㅛㅜㅝㅞㅟㅠㅡㅢㅣ"
    // index 0 = no final
    private const val JONG = " ㄱㄲㄳㄴㄵㄶㄷㄹㄺㄻㄼㄽㄾㄿㅀㅁㅂㅄㅅㅆㅇㅈㅊㅋㅌㅍㅎ"

    private val compoundJung: Map<Pair<Char, Char>, Char> = mapOf(
        ('ㅗ' to 'ㅏ') to 'ㅘ', ('ㅗ' to 'ㅐ') to 'ㅙ', ('ㅗ' to 'ㅣ') to 'ㅚ',
        ('ㅜ' to 'ㅓ') to 'ㅝ', ('ㅜ' to 'ㅔ') to 'ㅞ', ('ㅜ' to 'ㅣ') to 'ㅟ',
        ('ㅡ' to 'ㅣ') to 'ㅢ',
    )
    private val compoundJong: Map<Pair<Char, Char>, Char> = mapOf(
        ('ㄱ' to 'ㅅ') to 'ㄳ', ('ㄴ' to 'ㅈ') to 'ㄵ', ('ㄴ' to 'ㅎ') to 'ㄶ',
        ('ㄹ' to 'ㄱ') to 'ㄺ', ('ㄹ' to 'ㅁ') to 'ㄻ', ('ㄹ' to 'ㅂ') to 'ㄼ',
        ('ㄹ' to 'ㅅ') to 'ㄽ', ('ㄹ' to 'ㅌ') to 'ㄾ', ('ㄹ' to 'ㅍ') to 'ㄿ',
        ('ㄹ' to 'ㅎ') to 'ㅀ', ('ㅂ' to 'ㅅ') to 'ㅄ',
    )
    // Reverse: a compound final → (remaining final, jamo that moves off as next initial).
    private val splitJong: Map<Char, Pair<Char, Char>> =
        compoundJong.entries.associate { (k, v) -> v to (k.first to k.second) }

    private fun isCho(c: Char) = CHO.indexOf(c) >= 0
    private fun isJung(c: Char) = JUNG.indexOf(c) >= 0
    private fun isValidJong(c: Char) = JONG.indexOf(c) > 0

    private fun compose(cho: Char?, jung: Char?, jong: Char?): String {
        // Complete CV(C) block.
        if (cho != null && jung != null) {
            val ci = CHO.indexOf(cho)
            val ji = JUNG.indexOf(jung)
            val ki = if (jong != null) JONG.indexOf(jong) else 0
            if (ci >= 0 && ji >= 0 && ki >= 0) {
                return ((0xAC00 + (ci * 21 + ji) * 28 + ki).toChar()).toString()
            }
        }
        // Incomplete: emit whatever jamo we have, in order.
        val sb = StringBuilder()
        cho?.let { sb.append(it) }
        jung?.let { sb.append(it) }
        jong?.let { sb.append(it) }
        return sb.toString()
    }

    fun assemble(jamo: String): String {
        val out = StringBuilder()
        var cho: Char? = null
        var jung: Char? = null
        var jong: Char? = null

        fun flush() {
            out.append(compose(cho, jung, jong)); cho = null; jung = null; jong = null
        }

        for (j in jamo) {
            when {
                isCho(j) || isValidJong(j) -> {
                    val consonant = j
                    if (cho == null && jung == null) {
                        cho = consonant
                    } else if (cho != null && jung == null) {
                        // consonant after a lone initial → flush the lone initial, start anew
                        flush(); cho = consonant
                    } else if (jung != null && jong == null) {
                        if (isValidJong(consonant)) jong = consonant
                        else { flush(); cho = consonant }
                    } else { // cho+jung+jong present → try compound final, else new block
                        val comp = compoundJong[jong!! to consonant]
                        if (comp != null) jong = comp
                        else { flush(); cho = consonant }
                    }
                }
                isJung(j) -> {
                    val vowel = j
                    if (cho == null && jung == null) {
                        jung = vowel // standalone vowel
                    } else if (jung == null) {
                        jung = vowel // initial + vowel
                    } else if (jong == null) {
                        val comp = compoundJung[jung!! to vowel]
                        if (comp != null) jung = comp
                        else { flush(); jung = vowel } // new vowel-only block
                    } else {
                        // CVC + vowel → final moves to become the next block's initial.
                        val split = splitJong[jong]
                        if (split != null) {
                            jong = split.first
                            val moved = split.second
                            flush(); cho = moved; jung = vowel
                        } else {
                            val moved = jong
                            jong = null
                            flush(); cho = moved; jung = vowel
                        }
                    }
                }
                else -> { // non-jamo (shouldn't happen via the keyboard) — pass through
                    flush(); out.append(j)
                }
            }
        }
        out.append(compose(cho, jung, jong))
        return out.toString()
    }
}
