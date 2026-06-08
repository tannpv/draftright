import Foundation

/// Small built-in pinyin→hanzi seed so Chinese works the moment it's enabled,
/// before a downloadable pinyin pack lands (same TSV pipeline as Japanese).
/// Mirrors the Kotlin ChinesePinyinSeedDictionary.
public enum ChinesePinyinSeedDictionary {
    public static let dict: [String: [String]] = [
        "ni": ["你", "尼", "泥"],
        "hao": ["好", "号", "毫"],
        "nihao": ["你好"],
        "wo": ["我", "握", "卧"],
        "women": ["我们"],
        "ta": ["他", "她", "它", "塔"],
        "tamen": ["他们", "她们"],
        "shi": ["是", "时", "事", "十", "市"],
        "bu": ["不", "部", "步"],
        "de": ["的", "得", "地"],
        "le": ["了", "乐"],
        "hen": ["很", "狠"],
        "ai": ["爱", "哀"],
        "zhongguo": ["中国"],
        "zhongwen": ["中文"],
        "hanyu": ["汉语"],
        "xiexie": ["谢谢"],
        "zaijian": ["再见"],
        "ren": ["人", "认", "仁"],
        "da": ["大", "打", "答"],
        "xiao": ["小", "笑", "校"],
        "shui": ["水", "睡", "谁"],
        "huo": ["火", "或", "货"],
        "ri": ["日"],
        "yue": ["月", "约", "越"],
        "shan": ["山", "善"],
        "tian": ["天", "田", "甜"],
        "ming": ["名", "明", "命"],
        "zi": ["字", "自", "子"],
        "xie": ["写", "谢", "些"],
    ]
}
