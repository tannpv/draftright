import Foundation

/// In-bundle Vietnamese suggestion source used before the downloadable pack
/// arrives: the generated ~8.5k unigrams from `VietnameseWordList` plus the
/// hand-curated bigrams below.
///
/// Unigrams deliberately live in the generated file, not here — one source
/// (`tools/gen_vi_wordlist.py`) feeds both this and Android's
/// `res/raw/wordlist_vi.tsv`, byte-parity guarded (RULE #1). The bigrams have
/// no Swift generator yet; their Android mirror is
/// `res/raw/wordlist_vi_bigrams.tsv`, guarded by `check-vi-bigram-parity.py`.
///
/// Kept as Swift literals instead of bundled .tsv resources so the keyboard
/// extension needs no resource handling on first launch; the cost is static
/// data — negligible vs. the jetsam-cap memory headroom (~50 MB).
enum VietnameseBootstrapWordList {
    /// Common Vietnamese collocations for next-word prediction. **Mirror of
    /// `res/raw/wordlist_vi_bigrams.tsv` on Android** — the two are separate
    /// copies (Swift literal vs TSV resource) that MUST agree; `VietnameseBigramTests`
    /// pins a sample of (prev → top successor) so they can't drift silently
    /// (RULE #1). Values are relative weights, not corpus absolutes.
    static let bigrams: [String: [String: Int]] = [
        "xin": ["chào": 900, "lỗi": 700, "phép": 300, "cảm": 200],
        "cảm": ["ơn": 950, "thấy": 440],
        "không": ["có": 800, "được": 600, "phải": 550, "biết": 500, "sao": 400],
        "có": ["thể": 820, "phải": 400, "gì": 380, "người": 200],
        "rất": ["nhiều": 600, "tốt": 500, "đẹp": 450, "vui": 450, "mong": 300],
        "tôi": ["là": 780, "muốn": 600, "có": 560, "không": 540, "sẽ": 480, "cần": 360],
        "bạn": ["có": 620, "là": 500, "ơi": 480, "khỏe": 420],
        "của": ["tôi": 720, "bạn": 560, "mình": 400],
        "được": ["không": 620, "rồi": 520],
        "chào": ["bạn": 560, "buổi": 300],
        "buổi": ["sáng": 620, "tối": 560, "chiều": 420, "trưa": 360],
        "một": ["chút": 500, "lần": 460, "người": 440, "ngày": 420, "cái": 400],
        "hôm": ["nay": 780, "qua": 560],
        "bây": ["giờ": 720],
        "tại": ["sao": 680],
        "như": ["thế": 560, "vậy": 540],
        "thế": ["nào": 620],
        "cái": ["gì": 720, "này": 560, "đó": 480],
        "người": ["ta": 560, "việt": 300],
        "việt": ["nam": 900],
        "đi": ["làm": 560, "học": 540, "chơi": 460, "đâu": 440],
        "ăn": ["cơm": 620, "gì": 480],
        "uống": ["nước": 560],
        "yêu": ["em": 620, "anh": 600],
        "anh": ["yêu": 560],
        "em": ["yêu": 560],
        "chúc": ["mừng": 620, "ngủ": 400],
        "mừng": ["năm": 560],
        "năm": ["mới": 760],
        "ngủ": ["ngon": 620],
        "hẹn": ["gặp": 680],
        "gặp": ["lại": 720],
        "tạm": ["biệt": 700],
        "dạ": ["vâng": 560],
        "sinh": ["viên": 620, "nhật": 560],
        "học": ["sinh": 600, "tập": 480],
        "giáo": ["viên": 620],
        "điện": ["thoại": 760],
        "máy": ["tính": 620, "bay": 520],
        "số": ["điện": 480],
        "cần": ["phải": 560],
        "muốn": ["đi": 460, "ăn": 420],
        "thấy": ["rất": 300],
        "mình": ["là": 420, "có": 400],
    ]

    /// Cached InMemoryWordList — built once at first access. Engines built
    /// from this share the underlying storage so the ~8.5k-entry index isn't
    /// rebuilt per pack instance.
    static let wordList: InMemoryWordList =
        InMemoryWordList(words: VietnameseWordList.entries, bigrams: bigrams)
}
