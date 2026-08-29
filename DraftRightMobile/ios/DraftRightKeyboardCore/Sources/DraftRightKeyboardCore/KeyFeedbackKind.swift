import Foundation

/// The feedback-relevant class of a key press (#209, mirror of the Kotlin
/// `KeyFeedbackKind`). On Android each kind selects a distinct platform click
/// sound; iOS has a single system input click (`UIDevice.playInputClick`), so
/// the per-kind distinction lives in the **haptic** strength instead — but the
/// kinds themselves are the shared vocabulary both platforms route every key
/// through, and are parity-guarded (`check-keyfeedback-kind-parity.py`).
public enum KeyFeedbackKind {
    case char
    case space
    case delete
    case enter
    case other  // shift / layer switch / globe

    /// Haptic strength for this kind. Delete/enter get a firmer tick (a
    /// destructive or commit action); ordinary typing stays light — the same
    /// "important keys feel different" cue Samsung/Gboard give.
    public var impact: KeyFeedbackImpact {
        switch self {
        case .delete, .enter: return .medium
        case .char, .space, .other: return .light
        }
    }
}

/// Platform-neutral haptic strength (maps to `UIImpactFeedbackGenerator.FeedbackStyle`
/// in the extension). Kept out of the enum so the mapping is unit-testable without
/// UIKit.
public enum KeyFeedbackImpact {
    case light
    case medium
}
