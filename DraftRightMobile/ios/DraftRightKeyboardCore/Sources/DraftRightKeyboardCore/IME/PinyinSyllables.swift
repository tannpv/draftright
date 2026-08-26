import Foundation

/// Valid **toneless** Mandarin pinyin syllables — the segmentation oracle for
/// sentence-level pinyin (#211). A linguistic constant (the standard Hanyu Pinyin
/// syllabary), NOT our dictionary's coverage. Mirror of the Kotlin
/// `PinyinSyllables`; `scripts/check-pinyin-syllables-parity.py` asserts the two
/// copies agree so they can't drift.
public enum PinyinSyllables {

    /// Longest valid syllable is 6 chars ("zhuang"/"shuang"/"chuang").
    public static let maxLen = 6

    public static let all: Set<String> = [
        "a", "ai", "an", "ang", "ao", "e", "ei", "en", "eng", "er", "o", "ou",
        "ba", "bai", "ban", "bang", "bao", "bei", "ben", "beng", "bi", "bian",
        "biao", "bie", "bin", "bing", "bo", "bu",
        "pa", "pai", "pan", "pang", "pao", "pei", "pen", "peng", "pi", "pian",
        "piao", "pie", "pin", "ping", "po", "pou", "pu",
        "ma", "mai", "man", "mang", "mao", "me", "mei", "men", "meng", "mi",
        "mian", "miao", "mie", "min", "ming", "miu", "mo", "mou", "mu",
        "fa", "fan", "fang", "fei", "fen", "feng", "fo", "fou", "fu",
        "da", "dai", "dan", "dang", "dao", "de", "dei", "den", "deng", "di",
        "dian", "diao", "die", "ding", "diu", "dong", "dou", "du", "duan", "dui",
        "dun", "duo",
        "ta", "tai", "tan", "tang", "tao", "te", "teng", "ti", "tian", "tiao",
        "tie", "ting", "tong", "tou", "tu", "tuan", "tui", "tun", "tuo",
        "na", "nai", "nan", "nang", "nao", "ne", "nei", "nen", "neng", "ni",
        "nian", "niang", "niao", "nie", "nin", "ning", "niu", "nong", "nou", "nu",
        "nuan", "nuo", "nv", "nve",
        "la", "lai", "lan", "lang", "lao", "le", "lei", "leng", "li", "lia",
        "lian", "liang", "liao", "lie", "lin", "ling", "liu", "long", "lou", "lu",
        "luan", "lun", "luo", "lv", "lve",
        "ga", "gai", "gan", "gang", "gao", "ge", "gei", "gen", "geng", "gong",
        "gou", "gu", "gua", "guai", "guan", "guang", "gui", "gun", "guo",
        "ka", "kai", "kan", "kang", "kao", "ke", "kei", "ken", "keng", "kong",
        "kou", "ku", "kua", "kuai", "kuan", "kuang", "kui", "kun", "kuo",
        "ha", "hai", "han", "hang", "hao", "he", "hei", "hen", "heng", "hong",
        "hou", "hu", "hua", "huai", "huan", "huang", "hui", "hun", "huo",
        "ji", "jia", "jian", "jiang", "jiao", "jie", "jin", "jing", "jiong",
        "jiu", "ju", "juan", "jue", "jun",
        "qi", "qia", "qian", "qiang", "qiao", "qie", "qin", "qing", "qiong",
        "qiu", "qu", "quan", "que", "qun",
        "xi", "xia", "xian", "xiang", "xiao", "xie", "xin", "xing", "xiong",
        "xiu", "xu", "xuan", "xue", "xun",
        "zha", "zhai", "zhan", "zhang", "zhao", "zhe", "zhei", "zhen", "zheng",
        "zhi", "zhong", "zhou", "zhu", "zhua", "zhuai", "zhuan", "zhuang", "zhui",
        "zhun", "zhuo",
        "cha", "chai", "chan", "chang", "chao", "che", "chen", "cheng", "chi",
        "chong", "chou", "chu", "chua", "chuai", "chuan", "chuang", "chui",
        "chun", "chuo",
        "sha", "shai", "shan", "shang", "shao", "she", "shei", "shen", "sheng",
        "shi", "shou", "shu", "shua", "shuai", "shuan", "shuang", "shui", "shun",
        "shuo",
        "ran", "rang", "rao", "re", "ren", "reng", "ri", "rong", "rou", "ru",
        "rua", "ruan", "rui", "run", "ruo",
        "za", "zai", "zan", "zang", "zao", "ze", "zei", "zen", "zeng", "zi",
        "zong", "zou", "zu", "zuan", "zui", "zun", "zuo",
        "ca", "cai", "can", "cang", "cao", "ce", "cen", "ceng", "ci", "cong",
        "cou", "cu", "cuan", "cui", "cun", "cuo",
        "sa", "sai", "san", "sang", "sao", "se", "sen", "seng", "si", "song",
        "sou", "su", "suan", "sui", "sun", "suo",
        "ya", "yan", "yang", "yao", "ye", "yi", "yin", "ying", "yo", "yong",
        "you", "yu", "yuan", "yue", "yun",
        "wa", "wai", "wan", "wang", "wei", "wen", "weng", "wo", "wu",
    ]

    public static func isSyllable(_ s: String) -> Bool { all.contains(s) }
}
