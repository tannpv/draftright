import Foundation

/// How the rewrite panel is triggered.
///
/// This is the single source of truth for which trigger mechanisms are active,
/// decoupled from whether a hotkey combo happens to be set. Previously the mode
/// was inferred from `hotkeyString` being empty or not — a boolean, so only two
/// states were possible. An explicit enum adds the third (`both`) and keeps the
/// hotkey combo as pure data that survives a mode switch (#179).
enum TriggerMode: String, CaseIterable, Identifiable {
    /// The pencil button appears when text is selected.
    case pencil
    /// A global keyboard shortcut opens the panel.
    case hotkey
    /// Both at once — the pencil appears AND the shortcut fires.
    case both

    var id: String { rawValue }

    /// Whether the on-selection pencil button should be active in this mode.
    var usesPencil: Bool { self == .pencil || self == .both }

    /// Whether the global hotkey should be registered in this mode.
    var usesHotkey: Bool { self == .hotkey || self == .both }

    var displayName: String {
        switch self {
        case .pencil: return "Pencil"
        case .hotkey: return "Hotkey"
        case .both: return "Both"
        }
    }
}
