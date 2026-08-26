package com.draftright.keyboard.ime

/**
 * The Japanese 12-key flick (フリック) kana map (#212): each key is a gojūon row,
 * and a flick direction picks the vowel — tap=あ-dan, ←=い, ↑=う, →=え, ↓=お.
 *
 * This is a linguistic constant (the standard flick arrangement used by the
 * common JP mobile keyboards), one source of truth, mirrored 1:1 in the Swift
 * `FlickLayout` (parity-guarded). The resolved kana feeds the existing kana→kanji
 * candidate engine unchanged — flick is only a new *input* surface (RULE #1).
 */
object FlickLayout {

    /** Row-head kana (the key's tap output) → direction → kana. */
    val rows: Map<String, Map<FlickDirection, String>> = mapOf(
        "あ" to gojuon("あ", "い", "う", "え", "お"),
        "か" to gojuon("か", "き", "く", "け", "こ"),
        "さ" to gojuon("さ", "し", "す", "せ", "そ"),
        "た" to gojuon("た", "ち", "つ", "て", "と"),
        "な" to gojuon("な", "に", "ぬ", "ね", "の"),
        "は" to gojuon("は", "ひ", "ふ", "へ", "ほ"),
        "ま" to gojuon("ま", "み", "む", "め", "も"),
        // や row has only 3 kana: tap や, up ゆ, down よ.
        "や" to mapOf(
            FlickDirection.TAP to "や",
            FlickDirection.UP to "ゆ",
            FlickDirection.DOWN to "よ",
        ),
        "ら" to gojuon("ら", "り", "る", "れ", "ろ"),
        // わ row is special: tap わ, ← を, ↑ ん, → ー (chōonpu), ↓ 〜.
        "わ" to mapOf(
            FlickDirection.TAP to "わ",
            FlickDirection.LEFT to "を",
            FlickDirection.UP to "ん",
            FlickDirection.RIGHT to "ー",
            FlickDirection.DOWN to "〜",
        ),
    )

    /** The kana produced by flicking [direction] on the key whose tap output is
     *  [rowHead], or null when that key has no kana in that direction. */
    fun kanaFor(rowHead: String, direction: FlickDirection): String? =
        rows[rowHead]?.get(direction)

    /** tap=a, left=i, up=u, right=e, down=o — the uniform gojūon flick. */
    private fun gojuon(a: String, i: String, u: String, e: String, o: String) = mapOf(
        FlickDirection.TAP to a,
        FlickDirection.LEFT to i,
        FlickDirection.UP to u,
        FlickDirection.RIGHT to e,
        FlickDirection.DOWN to o,
    )
}
