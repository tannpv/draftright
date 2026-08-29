import Foundation

public struct KeyDef: Equatable {
    public let label: String
    public let code: Int
    public let widthWeight: CGFloat

    public init(_ label: String, _ code: Int, widthWeight: CGFloat = 1.0) {
        self.label = label
        self.code = code
        self.widthWeight = widthWeight
    }
}

/// Unicode scalar value used as the key code for a literal character key.
/// Shared by every LanguagePack so the layout files don't re-spell
/// `Int(Character("x").unicodeScalars.first!.value)` at each call site.
func keyCode(_ label: String) -> Int {
    Int(label.unicodeScalars.first!.value)
}

/// Builds character KeyDefs from single-character labels. Shared across
/// language packs so each one no longer redeclares its own `chars` helper.
func chars(_ labels: String...) -> [KeyDef] {
    labels.map { KeyDef($0, keyCode($0)) }
}

public protocol LanguagePack {
    var id: String { get }
    var displayName: String { get }
    var locale: Locale { get }
    var alphaRows: [[KeyDef]] { get }
    var symbols1Rows: [[KeyDef]] { get }
    var symbols2Rows: [[KeyDef]] { get }
    /// Dedicated numeric keypad for OTP/PIN/phone fields (#209). Defaults to the
    /// shared 1-9 / ABC 0 ← ↵ grid; packs rarely override it.
    var numericRows: [[KeyDef]] { get }
    var longPressAccents: [Character: [Character]] { get }
    func makeComposer() -> Composer?

    /// Suggestion engine shown in the candidate bar — Telex-aware trigram
    /// for Vietnamese, prefix-trigram for Latin scripts, RIME adapter for
    /// JP/ZH/KO, nil to render no bar at all (the default).
    ///
    /// Mirror of Kotlin `LanguagePack.candidateEngine()`. Returning the
    /// engine lazily means downloadable packs (RIME schemas, big word
    /// lists) can be installed AFTER the keyboard's first paint without
    /// a registry rebuild — the next syllable gets the new candidates.
    func makeCandidateEngine() -> CandidateEngine?

    /// Whether pressing space converts the live composing *reading* to the top
    /// candidate instead of committing the reading + a space. True for
    /// reading-conversion input (Japanese kana→kanji, Chinese pinyin→hanzi),
    /// the standard JP/ZH IME behavior. Mirror of Kotlin `convertsOnSpace`.
    var convertsOnSpace: Bool { get }
}

public extension LanguagePack {
    /// Default numeric keypad — the shared OTP/PIN grid.
    var numericRows: [[KeyDef]] { QwertyLayout.numericRows }
    /// Default: no composition (Latin packs type directly). JP/VI override.
    func makeComposer() -> Composer? { PassthroughComposer() }
    func makeCandidateEngine() -> CandidateEngine? { nil }
    /// Default: space is a literal space (Latin, Telex, Hangul). JP/ZH override.
    var convertsOnSpace: Bool { false }
}
