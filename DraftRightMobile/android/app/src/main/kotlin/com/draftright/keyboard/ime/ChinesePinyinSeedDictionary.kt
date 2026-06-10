package com.draftright.keyboard.ime

/**
 * Small built-in pinyin→hanzi seed so Chinese works the moment it's enabled,
 * before a downloadable pinyin pack lands (same TSV pipeline as Japanese).
 * Toneless full-syllable keys; candidates ranked by frequency.
 */
object ChinesePinyinSeedDictionary {
    val dict: Map<String, List<String>> = mapOf(
        "ni" to listOf("你", "尼", "泥"),
        "hao" to listOf("好", "号", "毫"),
        "nihao" to listOf("你好"),
        "wo" to listOf("我", "握", "卧"),
        "women" to listOf("我们"),
        "ta" to listOf("他", "她", "它", "塔"),
        "tamen" to listOf("他们", "她们"),
        "shi" to listOf("是", "时", "事", "十", "市"),
        "bu" to listOf("不", "部", "步"),
        "de" to listOf("的", "得", "地"),
        "le" to listOf("了", "乐"),
        "hen" to listOf("很", "狠"),
        "ai" to listOf("爱", "哀"),
        "zhongguo" to listOf("中国"),
        "zhongwen" to listOf("中文"),
        "hanyu" to listOf("汉语"),
        "xiexie" to listOf("谢谢"),
        "zaijian" to listOf("再见"),
        "ren" to listOf("人", "认", "仁"),
        "da" to listOf("大", "打", "答"),
        "xiao" to listOf("小", "笑", "校"),
        "shui" to listOf("水", "睡", "谁"),
        "huo" to listOf("火", "或", "货"),
        "ri" to listOf("日"),
        "yue" to listOf("月", "约", "越"),
        "shan" to listOf("山", "善"),
        "tian" to listOf("天", "田", "甜"),
        "ming" to listOf("名", "明", "命"),
        "zi" to listOf("字", "自", "子"),
        "xie" to listOf("写", "谢", "些"),
    )
}
