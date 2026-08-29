package com.draftright.keyboard.ime

/**
 * The 小゛゜ modifier for Japanese flick (#212, phase 3): cycles the last kana
 * through its small / dakuten / handakuten variants, the way the 大⇔小 key on a
 * real flick keyboard does — か→が→か, は→ば→ぱ→は, つ→っ→づ→つ, や→ゃ→や.
 * Without it じ/ば/ぱ/small kana can't be typed at all.
 *
 * The variant cycles are a cited linguistic constant (one source of truth,
 * mirror of the Swift `KanaModifier`, parity-guarded). Kana with no variants
 * (な/ま/ら rows, ん, を) return unchanged.
 */
object KanaModifier {

    /** Each row = a kana and its variants, in cycle order (base first). */
    private val cycles: List<List<String>> = listOf(
        listOf("あ", "ぁ"), listOf("い", "ぃ"), listOf("う", "ぅ", "ゔ"), listOf("え", "ぇ"), listOf("お", "ぉ"),
        listOf("か", "が"), listOf("き", "ぎ"), listOf("く", "ぐ"), listOf("け", "げ"), listOf("こ", "ご"),
        listOf("さ", "ざ"), listOf("し", "じ"), listOf("す", "ず"), listOf("せ", "ぜ"), listOf("そ", "ぞ"),
        listOf("た", "だ"), listOf("ち", "ぢ"), listOf("つ", "っ", "づ"), listOf("て", "で"), listOf("と", "ど"),
        listOf("は", "ば", "ぱ"), listOf("ひ", "び", "ぴ"), listOf("ふ", "ぶ", "ぷ"), listOf("へ", "べ", "ぺ"), listOf("ほ", "ぼ", "ぽ"),
        listOf("や", "ゃ"), listOf("ゆ", "ゅ"), listOf("よ", "ょ"),
        listOf("わ", "ゎ"),
    )

    private val next: Map<String, String> = buildMap {
        for (cycle in cycles) for (i in cycle.indices) put(cycle[i], cycle[(i + 1) % cycle.size])
    }

    /** The next variant of [kana] in its cycle, or [kana] unchanged if it has none. */
    fun cycle(kana: String): String = next[kana] ?: kana
}
