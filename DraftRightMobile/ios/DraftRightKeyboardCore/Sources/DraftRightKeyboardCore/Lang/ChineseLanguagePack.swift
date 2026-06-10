import Foundation

/// Chinese (中文) pinyin pack — type pinyin on the standard QWERTY;
/// `PinyinComposer` shows the pinyin live and `DictionaryCandidateEngine` offers
/// Hanzi candidates. Same shape as Japanese, only the dictionary differs.
/// Mirrors the Kotlin ChineseLanguagePack.
public struct ChineseLanguagePack: LanguagePack {
    public let id = "zh"
    public let displayName = "中文"
    public let locale = Locale(identifier: "zh")
    public let longPressAccents: [Character: [Character]] = [:]

    /// App Group container — set by KeyboardViewController, same as JP/EN.
    public static var appGroupContainer: URL?
    /// Matches the backend manifest's pack URL prefix (zh pack ships later).
    private static let packIdPrefix = "draftright-ime-zh"
    /// Cached engine — parse the pinyin pack once per session, not per keystroke.
    private static var cachedEngine: CandidateEngine?

    public init() {}

    public func makeComposer() -> Composer? { PinyinComposer() }

    public func makeCandidateEngine() -> CandidateEngine? {
        if let cached = Self.cachedEngine { return cached }
        let dict = DictPackResolver.loadOrFallback(
            appGroupContainer: Self.appGroupContainer,
            packIdPrefix: Self.packIdPrefix,
            fallback: { ChinesePinyinSeedDictionary.dict }
        )
        let engine = DictionaryCandidateEngine(dictionary: dict)
        Self.cachedEngine = engine
        return engine
    }

    public var alphaRows: [[KeyDef]] { QwertyLayout.alphaRows }
    public var symbols1Rows: [[KeyDef]] { QwertyLayout.symbols1Rows }
    public var symbols2Rows: [[KeyDef]] { QwertyLayout.symbols2Rows }
}
