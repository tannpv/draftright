import Foundation

/// One ranked suggestion shown in the candidate bar. `text` is what gets
/// committed when the user taps; `display` is what the bar renders (for CJK
/// scripts the two differ — display can show okurigana hints, the committed
/// text is the kanji+kana run). For Latin-script word prediction both are
/// the same string.
public struct Candidate: Equatable {
    public let text: String
    public let display: String

    public init(text: String, display: String? = nil) {
        self.text = text
        self.display = display ?? text
    }
}

/// Engine-agnostic seam between the keyboard UI and whatever produces
/// suggestions: a trigram engine for Latin scripts (mirrors the Android
/// implementation in `keyboard.ime.CandidateEngine`), a librime adapter
/// for CJK once Task 1 (RIME spike) clears, a no-op stub for languages
/// that ship without prediction.
///
/// The keyboard calls `suggest` every keystroke. Engines must return fast
/// — a 50 ms budget on an iPhone SE2 — and never block the main thread.
///
/// Per Rule #1: the protocol is intentionally minimal so adding a new
/// engine (e.g. an on-device LLM next year) doesn't ripple through the
/// keyboard, the candidate bar, or the language pack registry.
public protocol CandidateEngine {
    /// Produce up to `limit` candidates for the current input state.
    ///
    /// - Parameters:
    ///   - composing: Live composing buffer — what the user is actively
    ///                typing in the current word (Telex for Vietnamese,
    ///                romaji for Japanese, raw ASCII for English). Empty
    ///                means "no live word".
    ///   - previousTokens: Recent committed words (most-recent last), used
    ///                     for next-word prediction and n-gram context.
    ///                     Implementations may ignore them.
    ///   - limit: Hard cap on returned candidates. Most bars render 3–7;
    ///            engines can return fewer.
    func suggest(composing: String, previousTokens: [String], limit: Int) -> [Candidate]

    /// Free any data the engine memory-mapped or cached. Called when the
    /// IME service is shutting down or the user disables this language.
    func close()
}

public extension CandidateEngine {
    /// Default limit matches the Android side so iOS/Android UX stays
    /// in sync without each call site repeating the number.
    func suggest(composing: String, previousTokens: [String] = []) -> [Candidate] {
        suggest(composing: composing, previousTokens: previousTokens, limit: 7)
    }
    func close() {}
}
