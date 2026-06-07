package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult
import com.draftright.keyboard.Composer

class TelexComposer : Composer {

    private val buffer = StringBuilder()

    override fun onKey(char: Char): ComposeResult {
        if (buffer.length >= MAX_LEN) {
            val committed = buffer.toString()
            buffer.clear()
            buffer.append(char)
            return ComposeResult.Commit(committed + char)
        }

        if (!char.isLetter()) {
            return if (buffer.isEmpty()) {
                ComposeResult.PassThrough
            } else {
                val out = buffer.toString() + char
                buffer.clear()
                ComposeResult.Commit(out)
            }
        }

        val combined = tryCombine(buffer.toString(), char)
        if (combined != null) {
            buffer.clear()
            buffer.append(combined)
        } else {
            buffer.append(char)
        }
        return ComposeResult.Composing(buffer.toString())
    }

    override fun onBackspace(): ComposeResult {
        if (buffer.isEmpty()) return ComposeResult.PassThrough
        val stripped = stripOneLayer(buffer.toString())
        buffer.clear()
        buffer.append(stripped)
        return if (buffer.isEmpty()) ComposeResult.Consumed
        else ComposeResult.Composing(buffer.toString())
    }

    override fun reset() {
        buffer.clear()
    }

    override fun currentComposingText(): String = buffer.toString()

    companion object {
        const val MAX_LEN = 32

        fun tryCombine(buffer: String, incoming: Char): String? {
            if (buffer.isEmpty()) return null
            val low = incoming.lowercaseChar()

            // Tone marks (s/f/r/x/j) — apply if buffer contains a vowel-like
            // char (plain, special-marked, or already toned).
            if (low in TONE_MARKS && bufferHasTonableVowel(buffer)) {
                tryCancelTone(buffer, low, incoming)?.let { return it }
                return applyTone(buffer, low)
            }

            // 'w' has multiple meanings depending on the preceding chars.
            if (low == 'w') {
                tryCancelHornBreve(buffer, incoming)?.let { return it }
                return applyHornOrBreve(buffer, incoming.isUpperCase())
            }

            // dd → đ, or cancel đ back to d + literal d.
            if (low == 'd') {
                val last = buffer.last()
                if (last.lowercaseChar() == 'đ') {
                    return buffer.dropLast(1) + caseMap('d', last.isUpperCase()) + incoming
                }
                if (last.lowercaseChar() == 'd') {
                    return buffer.dropLast(1) + caseMap('đ', incoming.isUpperCase() || last.isUpperCase())
                }
                return null
            }

            // Double-vowel circumflex: aa/oo/ee. Re-type cancels back to base + literal.
            val replacement = when (low) {
                'a' -> 'â'
                'o' -> 'ô'
                'e' -> 'ê'
                else -> return null
            }
            val last = buffer.last()
            if (last.lowercaseChar() == replacement) {
                return buffer.dropLast(1) + caseMap(low, last.isUpperCase()) + incoming
            }
            if (last.lowercaseChar() == low) {
                return buffer.dropLast(1) + caseMap(replacement, incoming.isUpperCase() || last.isUpperCase())
            }

            // Lookback through up to MAX_TRAILING_CONS trailing consonants — lets
            // users type the modifier after the trailing consonant cluster, e.g.
            // "nguyen" + e → "nguyên" (skip the trailing 'n'). Same shape covers
            // the cancel case for re-typing.
            val targetIdx = findLastVowelThroughConsonants(buffer)
            if (targetIdx != null) {
                val targetChar = buffer[targetIdx]
                val targetLow = targetChar.lowercaseChar()
                if (targetLow == replacement) {
                    return buffer.substring(0, targetIdx) +
                        caseMap(low, targetChar.isUpperCase()) +
                        buffer.substring(targetIdx + 1) +
                        incoming
                }
                if (targetLow == low) {
                    return buffer.substring(0, targetIdx) +
                        caseMap(replacement, incoming.isUpperCase() || targetChar.isUpperCase()) +
                        buffer.substring(targetIdx + 1)
                }
            }
            return null
        }

        /**
         * Maximum trailing-consonant count the e/a/o/w modifier rules will scan
         * past when looking for their target vowel. 2 covers every Vietnamese
         * coda (ng, nh, ch, etc.) without crossing into the next syllable.
         */
        private const val MAX_TRAILING_CONS = 2

        /**
         * Scan from the end of [buffer] backwards; return the index of the last
         * vowel-like char, but only if at most [MAX_TRAILING_CONS] consonants
         * precede it. Used by tryCombine to support modifiers typed AFTER the
         * syllable's trailing consonants (e.g. "nguyen" + e, "truong" + w).
         */
        private fun findLastVowelThroughConsonants(buffer: String): Int? {
            var cons = 0
            for (i in buffer.indices.reversed()) {
                if (TelexState.isVowelLike(buffer[i])) return i
                cons++
                if (cons > MAX_TRAILING_CONS) return null
            }
            return null
        }

        private fun bufferHasTonableVowel(buffer: String): Boolean {
            return buffer.any {
                TelexState.isVowelLike(it) || UNTONE.containsKey(it.lowercaseChar())
            }
        }

        private fun tryCancelTone(buffer: String, toneChar: Char, incoming: Char): String? {
            val toneIdx = TONE_INDEX[toneChar] ?: return null
            // Scan right-to-left so the most recent tone gets canceled.
            for (i in buffer.indices.reversed()) {
                val c = buffer[i]
                val baseRoot = stripToneFromChar(c.lowercaseChar()) ?: continue
                val row = TONE_ROWS_LOWER[baseRoot] ?: continue
                if (row[toneIdx] == c.lowercaseChar()) {
                    val untoned = caseMap(baseRoot, c.isUpperCase())
                    return buffer.substring(0, i) + untoned + buffer.substring(i + 1) + incoming
                }
            }
            return null
        }

        private fun tryCancelHornBreve(buffer: String, incoming: Char): String? {
            if (buffer.isEmpty()) return null
            // uow cluster cancel: ươ → uo + literal w.
            if (buffer.length >= 2) {
                val twoBack = buffer[buffer.length - 2]
                val oneBack = buffer[buffer.length - 1]
                if (twoBack.lowercaseChar() == 'ư' && oneBack.lowercaseChar() == 'ơ') {
                    val u2 = caseMap('u', twoBack.isUpperCase())
                    val o2 = caseMap('o', oneBack.isUpperCase())
                    return buffer.dropLast(2) + u2 + o2 + incoming
                }
            }
            val last = buffer.last()
            val unmarked: Char? = when (last.lowercaseChar()) {
                'ă' -> 'a'
                'ơ' -> 'o'
                'ư' -> 'u'
                else -> null
            }
            if (unmarked != null) {
                return buffer.dropLast(1) + caseMap(unmarked, last.isUpperCase()) + incoming
            }
            return null
        }

        private fun applyHornOrBreve(buffer: String, wIsUpper: Boolean): String? {
            // A "uo" pair anywhere in the trailing vowel cluster becomes "ươ" —
            // even when another vowel follows it. This covers "uo" at the end
            // ("ruo"+w → "rươ"), before a coda ("truong"+w → "trương"), AND with
            // a trailing vowel ("ruou"+w → "rươu"/rượu, "nguoi"+w → "ngươi"/
            // người, "huou"+w → "hươu"). The single-vowel rules below would
            // otherwise horn the trailing vowel instead.
            val cluster = findLastVowelCluster(buffer)
            if (cluster != null) {
                for (i in cluster.first until cluster.last) {
                    if (buffer[i].lowercaseChar() == 'u' && buffer[i + 1].lowercaseChar() == 'o') {
                        val u2 = caseMap('ư', buffer[i].isUpperCase() || wIsUpper)
                        val o2 = caseMap('ơ', buffer[i + 1].isUpperCase() || wIsUpper)
                        return buffer.substring(0, i) + u2 + o2 + buffer.substring(i + 2)
                    }
                }
            }

            // Single horn/breve on the immediate last vowel: a→ă, o→ơ, u→ư.
            val last = buffer.last()
            val singleReplacement = when (last.lowercaseChar()) {
                'a' -> 'ă'
                'o' -> 'ơ'
                'u' -> 'ư'
                else -> null
            }
            if (singleReplacement != null) {
                return buffer.dropLast(1) + caseMap(singleReplacement, last.isUpperCase() || wIsUpper)
            }

            // Lookback through trailing consonants — lets users type 'w' after
            // the syllable's coda for the single-vowel case.
            val vowelIdx = findLastVowelThroughConsonants(buffer) ?: return null
            val vowelChar = buffer[vowelIdx]
            val lookbackReplacement = when (vowelChar.lowercaseChar()) {
                'a' -> 'ă'
                'o' -> 'ơ'
                'u' -> 'ư'
                else -> return null
            }
            return buffer.substring(0, vowelIdx) +
                caseMap(lookbackReplacement, vowelChar.isUpperCase() || wIsUpper) +
                buffer.substring(vowelIdx + 1)
        }

        private fun applyTone(buffer: String, toneChar: Char): String {
            val cluster = findLastVowelCluster(buffer) ?: return buffer
            val start = cluster.first
            val endInclusive = cluster.last
            val clusterLen = endInclusive - start + 1
            val hasTrailingConsonant = endInclusive < buffer.length - 1

            // Auto-promote diphthongs that take tone on the second vowel.
            // ie/uo/ye + (consonant or end) → iê/uô/yê before applying tone.
            if (clusterLen == 2 && !TelexState.isSpecialVowel(buffer[endInclusive])) {
                val first = buffer[start].lowercaseChar()
                val second = buffer[endInclusive].lowercaseChar()
                val promoted: Char? = when {
                    first == 'i' && second == 'e' -> 'ê'
                    first == 'y' && second == 'e' -> 'ê'
                    first == 'u' && second == 'o' -> 'ô'
                    else -> null
                }
                if (promoted != null) {
                    val promotedChar = caseMap(promoted, buffer[endInclusive].isUpperCase())
                    val withPromoted = buffer.substring(0, endInclusive) +
                        promotedChar + buffer.substring(endInclusive + 1)
                    return applyToneAt(withPromoted, endInclusive, toneChar)
                }
            }

            // Auto-promote 3-vowel clusters that take tone on the middle (uoi → uôi, ieu → iêu).
            if (clusterLen == 3 && !TelexState.isSpecialVowel(buffer[start + 1])) {
                val first = buffer[start].lowercaseChar()
                val mid = buffer[start + 1].lowercaseChar()
                val last = buffer[endInclusive].lowercaseChar()
                val promoted: Char? = when {
                    first == 'u' && mid == 'o' && last == 'i' -> 'ô'
                    first == 'i' && mid == 'e' && last == 'u' -> 'ê'
                    first == 'y' && mid == 'e' && last == 'u' -> 'ê'
                    else -> null
                }
                if (promoted != null) {
                    val promotedChar = caseMap(promoted, buffer[start + 1].isUpperCase())
                    val withPromoted = buffer.substring(0, start + 1) +
                        promotedChar + buffer.substring(start + 2)
                    return applyToneAt(withPromoted, start + 1, toneChar)
                }
            }

            // Auto-promote 3-vowel clusters that take tone on the LAST vowel
            // (uye + trailing consonant → uyê: e.g. "nguyen" + x → "nguyễn").
            // Distinct from the mid-promote group above so each promotion rule
            // stays explicit instead of being woven into the picker logic.
            if (clusterLen == 3 && hasTrailingConsonant && !TelexState.isSpecialVowel(buffer[endInclusive])) {
                val first = buffer[start].lowercaseChar()
                val mid = buffer[start + 1].lowercaseChar()
                val last = buffer[endInclusive].lowercaseChar()
                val promoted: Char? = when {
                    first == 'u' && mid == 'y' && last == 'e' -> 'ê'
                    else -> null
                }
                if (promoted != null) {
                    val promotedChar = caseMap(promoted, buffer[endInclusive].isUpperCase())
                    val withPromoted = buffer.substring(0, endInclusive) +
                        promotedChar + buffer.substring(endInclusive + 1)
                    return applyToneAt(withPromoted, endInclusive, toneChar)
                }
            }

            val targetIdx = pickToneVowelIndex(buffer, start, endInclusive, hasTrailingConsonant)
            return applyToneAt(buffer, targetIdx, toneChar)
        }

        private fun pickToneVowelIndex(
            buffer: String,
            start: Int,
            endInclusive: Int,
            hasTrailingConsonant: Boolean,
        ): Int {
            val len = endInclusive - start + 1
            if (len >= 3) {
                // A circumflex/horn/breve vowel takes the tone. When there
                // are two (e.g. "ươi"), the tone goes on the LAST one — ơ in
                // ươ, ê in uyê. Otherwise default to the middle vowel.
                for (i in endInclusive downTo start) {
                    if (TelexState.isSpecialVowel(buffer[i])) return i
                }
                return start + 1
            }
            if (len == 2) {
                val first = buffer[start]
                val second = buffer[endInclusive]
                if (TelexState.isSpecialVowel(second)) return endInclusive
                if (TelexState.isSpecialVowel(first)) return start
                return if (hasTrailingConsonant) endInclusive else start
            }
            return start
        }

        private fun findLastVowelCluster(body: String): IntRange? {
            var end = body.length
            while (end > 0 && !TelexState.isVowelLike(body[end - 1])) end--
            val clusterEnd = end
            while (end > 0 && TelexState.isVowelLike(body[end - 1])) end--
            val clusterStart = end
            return if (clusterStart < clusterEnd) clusterStart..(clusterEnd - 1) else null
        }

        private fun applyToneAt(buffer: String, idx: Int, toneChar: Char): String {
            val baseChar = buffer[idx]
            val toned = applyToneToChar(baseChar, toneChar) ?: return buffer
            return buffer.substring(0, idx) + toned + buffer.substring(idx + 1)
        }

        private fun applyToneToChar(base: Char, tone: Char): Char? {
            val toneIdx = TONE_INDEX[tone] ?: return null
            val baseLower = base.lowercaseChar()

            // If base is already toned, strip its tone first.
            val baseRoot = stripToneFromChar(baseLower) ?: baseLower
            val row = TONE_ROWS_LOWER[baseRoot] ?: return null
            val toned = row[toneIdx]
            return if (base.isUpperCase()) toned.uppercaseChar() else toned
        }

        fun stripOneLayer(buffer: String): String {
            if (buffer.isEmpty()) return ""
            val last = buffer.last()
            val rest = buffer.dropLast(1)
            val isUpper = last.isUpperCase()

            // 1. Strip tone if last char is toned.
            val untoned = stripToneFromChar(last.lowercaseChar())
            if (untoned != null) {
                return rest + caseMap(untoned, isUpper)
            }
            // 2. Strip diacritic mark (ă/â/ê/ô/ơ/ư/đ → base).
            val unmarked = UNMARK[last.lowercaseChar()]
            if (unmarked != null) {
                return rest + caseMap(unmarked, isUpper)
            }
            // 3. Drop the char.
            return rest
        }

        private fun stripToneFromChar(c: Char): Char? = UNTONE[c]

        private fun caseMap(c: Char, upper: Boolean): Char =
            if (upper) c.uppercaseChar() else c

        private val TONE_MARKS = setOf('s', 'f', 'r', 'x', 'j')
        private val TONE_INDEX = mapOf('s' to 0, 'f' to 1, 'r' to 2, 'x' to 3, 'j' to 4)

        private val TONE_ROWS_LOWER: Map<Char, CharArray> = mapOf(
            'a' to charArrayOf('á', 'à', 'ả', 'ã', 'ạ'),
            'ă' to charArrayOf('ắ', 'ằ', 'ẳ', 'ẵ', 'ặ'),
            'â' to charArrayOf('ấ', 'ầ', 'ẩ', 'ẫ', 'ậ'),
            'e' to charArrayOf('é', 'è', 'ẻ', 'ẽ', 'ẹ'),
            'ê' to charArrayOf('ế', 'ề', 'ể', 'ễ', 'ệ'),
            'i' to charArrayOf('í', 'ì', 'ỉ', 'ĩ', 'ị'),
            'o' to charArrayOf('ó', 'ò', 'ỏ', 'õ', 'ọ'),
            'ô' to charArrayOf('ố', 'ồ', 'ổ', 'ỗ', 'ộ'),
            'ơ' to charArrayOf('ớ', 'ờ', 'ở', 'ỡ', 'ợ'),
            'u' to charArrayOf('ú', 'ù', 'ủ', 'ũ', 'ụ'),
            'ư' to charArrayOf('ứ', 'ừ', 'ử', 'ữ', 'ự'),
            'y' to charArrayOf('ý', 'ỳ', 'ỷ', 'ỹ', 'ỵ'),
        )

        // Reverse map: any toned char → its untoned root.
        private val UNTONE: Map<Char, Char> = buildMap {
            for ((base, row) in TONE_ROWS_LOWER) {
                for (toned in row) put(toned, base)
            }
        }

        // Mark removal: special vowels and đ → bare ASCII root.
        private val UNMARK = mapOf(
            'ă' to 'a', 'â' to 'a',
            'ê' to 'e',
            'ô' to 'o', 'ơ' to 'o',
            'ư' to 'u',
            'đ' to 'd',
        )
    }
}
