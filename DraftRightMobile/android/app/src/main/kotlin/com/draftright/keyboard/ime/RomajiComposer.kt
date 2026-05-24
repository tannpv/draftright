package com.draftright.keyboard.ime

/**
 * Deterministic rōmaji → hiragana composer (offline front-half of Japanese
 * input). Mirrors the iOS DraftRightKeyboardCore RomajiComposer. Accumulates
 * typed rōmaji and converts greedily to kana, keeping any not-yet-resolvable
 * tail as rōmaji. The kana is what the RIME engine converts to kanji.
 *
 * Rules: base syllables, y-glides (kya/sha/cho), irregulars (shi/chi/tsu/fu/ji),
 * small-tsu sokuon (doubled consonant → っ), moraic n (n+consonant → ん keeping
 * the consonant; n' forces a standalone ん).
 */
class RomajiComposer {
    private val kana = StringBuilder()
    private var pending = ""

    /** Current composing text = resolved kana + unresolved rōmaji tail. */
    fun text(): String = kana.toString() + pending

    fun reset() {
        kana.setLength(0)
        pending = ""
    }

    /** Feed one or more rōmaji characters; returns the current composing text. */
    fun feed(s: String): String {
        for (ch in s.lowercase()) {
            pending += ch
            resolve()
        }
        return text()
    }

    private fun resolve() {
        while (pending.isNotEmpty()) {
            val c = pending
            // Sokuon: doubled non-"n" consonant → っ + drop one.
            if (c.length >= 2 && c[0] == c[1] && isConsonant(c[0]) && c[0] != 'n') {
                kana.append("っ")
                pending = c.substring(1)
                continue
            }
            // Moraic n: n + non-y consonant (incl. another n) → ん, keep that
            // consonant for the next mora. n' forces a standalone ん.
            if (c[0] == 'n' && c.length >= 2) {
                val second = c[1]
                if (second == '\'') {
                    kana.append("ん")
                    pending = c.substring(2)
                    continue
                }
                if (isConsonant(second) && second != 'y') {
                    kana.append("ん")
                    pending = c.substring(1)
                    continue
                }
            }
            // Longest-match (3 → 1).
            var matched = false
            for (len in minOf(3, c.length) downTo 1) {
                val unit = TABLE[c.substring(0, len)]
                if (unit != null) {
                    kana.append(unit)
                    pending = c.substring(len)
                    matched = true
                    break
                }
            }
            if (matched) continue
            // Could still grow into a key → wait. Otherwise flush leading char.
            if (TABLE.keys.any { it.startsWith(pending) }) break
            kana.append(c[0])
            pending = c.substring(1)
        }
    }

    companion object {
        private fun isConsonant(ch: Char): Boolean = ch in "bcdfghjklmnpqrstvwxyz"

        private val TABLE: Map<String, String> = mapOf(
            "a" to "あ", "i" to "い", "u" to "う", "e" to "え", "o" to "お",
            "ka" to "か", "ki" to "き", "ku" to "く", "ke" to "け", "ko" to "こ",
            "ga" to "が", "gi" to "ぎ", "gu" to "ぐ", "ge" to "げ", "go" to "ご",
            "kya" to "きゃ", "kyu" to "きゅ", "kyo" to "きょ",
            "gya" to "ぎゃ", "gyu" to "ぎゅ", "gyo" to "ぎょ",
            "sa" to "さ", "si" to "し", "shi" to "し", "su" to "す", "se" to "せ", "so" to "そ",
            "za" to "ざ", "zi" to "じ", "ji" to "じ", "zu" to "ず", "ze" to "ぜ", "zo" to "ぞ",
            "sha" to "しゃ", "shu" to "しゅ", "sho" to "しょ",
            "sya" to "しゃ", "syu" to "しゅ", "syo" to "しょ",
            "ja" to "じゃ", "ju" to "じゅ", "jo" to "じょ",
            "ta" to "た", "ti" to "ち", "chi" to "ち", "tu" to "つ", "tsu" to "つ", "te" to "て", "to" to "と",
            "da" to "だ", "di" to "ぢ", "du" to "づ", "de" to "で", "do" to "ど",
            "cha" to "ちゃ", "chu" to "ちゅ", "cho" to "ちょ",
            "tya" to "ちゃ", "tyu" to "ちゅ", "tyo" to "ちょ",
            "na" to "な", "ni" to "に", "nu" to "ぬ", "ne" to "ね", "no" to "の",
            "nya" to "にゃ", "nyu" to "にゅ", "nyo" to "にょ",
            "ha" to "は", "hi" to "ひ", "hu" to "ふ", "fu" to "ふ", "he" to "へ", "ho" to "ほ",
            "ba" to "ば", "bi" to "び", "bu" to "ぶ", "be" to "べ", "bo" to "ぼ",
            "pa" to "ぱ", "pi" to "ぴ", "pu" to "ぷ", "pe" to "ぺ", "po" to "ぽ",
            "hya" to "ひゃ", "hyu" to "ひゅ", "hyo" to "ひょ",
            "bya" to "びゃ", "byu" to "びゅ", "byo" to "びょ",
            "pya" to "ぴゃ", "pyu" to "ぴゅ", "pyo" to "ぴょ",
            "fa" to "ふぁ", "fi" to "ふぃ", "fe" to "ふぇ", "fo" to "ふぉ",
            "ma" to "ま", "mi" to "み", "mu" to "む", "me" to "め", "mo" to "も",
            "mya" to "みゃ", "myu" to "みゅ", "myo" to "みょ",
            "ya" to "や", "yu" to "ゆ", "yo" to "よ",
            "ra" to "ら", "ri" to "り", "ru" to "る", "re" to "れ", "ro" to "ろ",
            "rya" to "りゃ", "ryu" to "りゅ", "ryo" to "りょ",
            "wa" to "わ", "wo" to "を", "nn" to "ん",
            "-" to "ー", "," to "、", "." to "。",
        )
    }
}
