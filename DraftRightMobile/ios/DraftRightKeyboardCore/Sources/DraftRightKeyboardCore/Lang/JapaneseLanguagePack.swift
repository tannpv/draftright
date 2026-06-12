import Foundation

/// Japanese (rōmaji → kana → kanji) language pack. The layout is the standard
/// ASCII QWERTY (rōmaji is typed in Latin letters); the intelligence comes from
/// `makeCandidateEngine()` → `JapaneseDictionaryEngine`, which converts the
/// composing rōmaji to kana and offers kanji candidates in the candidate bar.
///
/// Ships with a small built-in seed dictionary so it works on enable; the full
/// reading→kanji dictionary arrives later as a downloadable pack.
public struct JapaneseLanguagePack: LanguagePack {
    public let id = "ja"
    public let displayName = "日本語"
    public let locale = Locale(identifier: "ja")
    public let longPressAccents: [Character: [Character]] = [:]

    /// App Group container — set by KeyboardViewController, same pattern as English.
    public static var appGroupContainer: URL?

    /// Matches the backend manifest's pack URL prefix.
    private static let packIdPrefix = "draftright-ime-ja"

    public init() {}

    /// Cached engine + the pack identity it was built from. makeCandidateEngine()
    /// is called on EVERY keystroke and the dictionary is 700k+ entries (~27 MB),
    /// so we keep the parsed engine across keystrokes — but key the cache on the
    /// resolved pack URL (its filename encodes the version) so installing a newer
    /// pack mid-session rebuilds instead of serving the stale dict (issue #10).
    /// The 27 MB parse still happens only on an actual pack change, not per key.
    private static var cachedEngine: CandidateEngine?
    private static var cachedKey: String?

    /// Rōmaji→kana composer; its kana buffer drives the candidate engine.
    public func makeComposer() -> Composer? { RomajiKanaComposer() }

    public func makeCandidateEngine() -> CandidateEngine? {
        let key = DictPackResolver.resolvedPackURL(
            appGroupContainer: Self.appGroupContainer,
            packIdPrefix: Self.packIdPrefix
        )?.path ?? "seed"
        if key == Self.cachedKey, let cached = Self.cachedEngine { return cached }
        let dict = DictPackResolver.loadOrFallback(
            appGroupContainer: Self.appGroupContainer,
            packIdPrefix: Self.packIdPrefix,
            fallback: { JapaneseSeedDictionary.dict }
        )
        let engine = DictionaryCandidateEngine(dictionary: dict)
        Self.cachedEngine = engine
        Self.cachedKey = key
        return engine
    }

    // Rōmaji is typed on the standard ASCII QWERTY (shared with English).
    public var alphaRows: [[KeyDef]] { QwertyLayout.alphaRows }
    public var symbols1Rows: [[KeyDef]] { QwertyLayout.symbols1Rows }
    public var symbols2Rows: [[KeyDef]] { QwertyLayout.symbols2Rows }
}
