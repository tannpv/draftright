package com.draftright.keyboard.ime

/**
 * Fuzzy-pinyin folding (#211): collapses the pinyin distinctions many speakers
 * (esp. southern Mandarin) don't reliably produce, so a sloppy spelling still
 * finds the word — "zongguo" matches 中国 (zhongguo), "si" matches 是 (shi).
 *
 * The standard fuzzy pairs: retroflex↔dental initials (zh/ch/sh → z/c/s) and
 * the nasal finals (ang/eng/ing → an/en/in). Applied to BOTH the dictionary
 * keys and the query so their folded forms meet in the middle — one function,
 * one rule set (RULE #1). Mirror of the Swift `PinyinFuzzy`.
 */
object PinyinFuzzy {

    fun fold(pinyin: String): String {
        var s = pinyin.lowercase()
        // Initials first (2-letter), then finals (3-letter). They don't overlap,
        // so order only matters for readability.
        s = s.replace("zh", "z").replace("ch", "c").replace("sh", "s")
        s = s.replace("ang", "an").replace("eng", "en").replace("ing", "in")
        return s
    }
}
