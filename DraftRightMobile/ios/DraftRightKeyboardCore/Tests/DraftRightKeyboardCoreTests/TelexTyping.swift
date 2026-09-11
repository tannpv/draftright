@testable import DraftRightKeyboardCore

/// Feeds a key string to a fresh `TelexComposer` one character at a time and
/// returns what the user would see. Shared by every Telex suite that works in
/// keystrokes (order-free, vectors, exhaustive) so the driver itself is one
/// source of truth.
enum TelexTyping {
    static func type(_ keys: String) -> String {
        let composer = TelexComposer()
        var last: ComposeResult = .passThrough
        for k in keys { last = composer.onKey(k) }
        switch last {
        case .composing(let s), .commit(let s): return s
        default: return composer.currentComposingText()
        }
    }
}
