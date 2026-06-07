package com.draftright.keyboard.ime

/**
 * Small built-in reading→kanji seed so Japanese works the moment it's enabled,
 * before the full downloadable dictionary pack lands. Mirrors the Swift
 * JapaneseSeedDictionary. NOT GPL (the full pack will use a permissive source,
 * Mozc/jawiki, delivered as a download).
 */
object JapaneseSeedDictionary {
    val dict: Map<String, List<String>> = mapOf(
        "にほん" to listOf("日本"),
        "にほんご" to listOf("日本語"),
        "かんじ" to listOf("漢字", "幹事", "感じ"),
        "ひらがな" to listOf("平仮名"),
        "かたかな" to listOf("片仮名"),
        "わたし" to listOf("私"),
        "あなた" to listOf("貴方"),
        "ともだち" to listOf("友達"),
        "がっこう" to listOf("学校"),
        "せんせい" to listOf("先生"),
        "がくせい" to listOf("学生"),
        "ほん" to listOf("本"),
        "みず" to listOf("水"),
        "き" to listOf("木", "気"),
        "つき" to listOf("月"),
        "ひと" to listOf("人"),
        "くに" to listOf("国"),
        "ねこ" to listOf("猫"),
        "いぬ" to listOf("犬"),
        "さかな" to listOf("魚"),
        "やま" to listOf("山"),
        "かわ" to listOf("川"),
        "うみ" to listOf("海"),
        "そら" to listOf("空"),
        "あめ" to listOf("雨"),
        "はな" to listOf("花", "鼻"),
        "たべる" to listOf("食べる"),
        "のむ" to listOf("飲む"),
        "みる" to listOf("見る"),
        "いく" to listOf("行く"),
        "くる" to listOf("来る"),
        "いう" to listOf("言う"),
        "ありがとう" to listOf("有難う"),
        "なまえ" to listOf("名前"),
        "でんわ" to listOf("電話"),
        "じかん" to listOf("時間"),
        "でんしゃ" to listOf("電車"),
        "とうきょう" to listOf("東京"),
        "おおさか" to listOf("大阪"),
        "きょう" to listOf("今日", "京"),
        "にちようび" to listOf("日曜日"),
    )
}
