import Foundation

/// No-op composer for languages that type directly with no composition
/// (English and the other Latin packs). Every key/backspace passes through to
/// the input field unchanged; there is no composing buffer.
///
/// Lets every LanguagePack return a real Composer (uniform pipeline) instead of
/// nil. Behaviour matches the previous nil path (KeyboardController maps
/// passThrough → commit char / deleteOne).
public final class PassthroughComposer: Composer {
    public init() {}
    public func onKey(_ char: Character) -> ComposeResult { .passThrough }
    public func onBackspace() -> ComposeResult { .passThrough }
    public func reset() {}
    public func currentComposingText() -> String { "" }
}
