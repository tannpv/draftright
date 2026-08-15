import Foundation

/// How the rewrite panel is triggered — the pencil button on selection, or a
/// global hotkey. Mutually exclusive: exactly one is active at a time.
///
/// This is the single source of truth, decoupled from whether a hotkey combo
/// happens to be set: switching to the pencil keeps the saved combo, so
/// switching back doesn't lose it (the reason this is an explicit enum rather
/// than a boolean inferred from `hotkeyString`).
enum TriggerMode: String, CaseIterable, Identifiable {
    /// The pencil button appears when text is highlighted by dragging.
    case pencil
    /// A global keyboard shortcut opens the panel.
    case hotkey

    var id: String { rawValue }

    /// Whether the on-selection pencil button should be active in this mode.
    var usesPencil: Bool { self == .pencil }

    /// Whether the global hotkey should be registered in this mode.
    var usesHotkey: Bool { self == .hotkey }

    var displayName: String {
        switch self {
        case .pencil: return "Pencil"
        case .hotkey: return "Hotkey"
        }
    }
}
