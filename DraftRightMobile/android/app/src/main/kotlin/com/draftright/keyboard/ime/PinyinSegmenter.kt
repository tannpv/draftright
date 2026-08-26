package com.draftright.keyboard.ime

/**
 * Splits a run-together pinyin string into syllables (#211, sentence-level
 * pinyin) — e.g. "woshi" → [wo, shi], "xianggang" → [xiang, gang]. This is the
 * primitive that turns "type the whole sentence's pinyin, pick segmented hanzi"
 * into reality; the candidate engine looks each syllable up in the dictionary.
 *
 * Backtracking longest-match: at each position it tries the LONGEST valid
 * syllable first (so "xian" segments as [xian], not [xi, an]) and backtracks
 * if the remainder can't be segmented. Returns null when no full segmentation
 * exists (partial/invalid pinyin) so the caller falls back to the raw string.
 *
 * Pure — unit-tested without any Android dependency, and mirrored 1:1 by the
 * Swift `PinyinSegmenter`.
 */
object PinyinSegmenter {

    fun segment(pinyin: String): List<String>? {
        val s = pinyin.lowercase()
        if (s.isEmpty()) return emptyList()
        return segFrom(s, 0, HashMap())
    }

    // memo[start] present → already solved (value may be null = "no segmentation
    // from here"); absent → not yet tried. Bounds the worst case on long input.
    private fun segFrom(s: String, start: Int, memo: HashMap<Int, List<String>?>): List<String>? {
        if (start == s.length) return emptyList()
        if (memo.containsKey(start)) return memo[start]
        val maxEnd = minOf(start + PinyinSyllables.MAX_LEN, s.length)
        // Longest candidate syllable first.
        for (end in maxEnd downTo start + 1) {
            val syllable = s.substring(start, end)
            if (!PinyinSyllables.isSyllable(syllable)) continue
            val rest = segFrom(s, end, memo)
            if (rest != null) {
                val result = ArrayList<String>(rest.size + 1)
                result.add(syllable)
                result.addAll(rest)
                memo[start] = result
                return result
            }
        }
        memo[start] = null
        return null
    }
}
