import UIKit
import DraftRightKeyboardCore

/// Samsung-parity key feedback for the iOS keyboard (#209): a system input click
/// on every key, plus a short haptic tick whose strength depends on the key kind
/// (see `KeyFeedbackKind.impact`). The single chokepoint every key routes through
/// — RULE #1: feedback is cross-cutting, so it lives here and is called from each
/// delegate method, not copied per key.
///
/// - The click uses `UIDevice.playInputClick()`, which is silent unless the user
///   enabled keyboard clicks and the input view conforms to
///   `UIInputViewAudioFeedback` (see `KeyboardViewController`). No settings gate
///   to maintain.
/// - Haptics require **Full Access** in a keyboard extension; without it we
///   degrade to click-only rather than doing nothing or crashing. The kind→sound
///   mapping is unit-tested (`KeyFeedbackKindTests`); the firing is verified
///   on-device.
struct KeyFeedback {
    /// Whether the extension has Full Access (from `UIInputViewController.hasFullAccess`).
    /// Read at fire time so toggling access mid-session is honoured.
    let hasFullAccess: () -> Bool

    /// Reused generators so each press doesn't allocate; `prepare()` on touch keeps
    /// the Taptic Engine warm for a crisp tick.
    private let lightHaptic = UIImpactFeedbackGenerator(style: .light)
    private let mediumHaptic = UIImpactFeedbackGenerator(style: .medium)

    init(hasFullAccess: @escaping () -> Bool) {
        self.hasFullAccess = hasFullAccess
    }

    /// Fire click + haptic for a key of the given [kind]. Safe on every press.
    func fire(_ kind: KeyFeedbackKind) {
        UIDevice.current.playInputClick()
        guard hasFullAccess() else { return }
        switch kind.impact {
        case .light: lightHaptic.impactOccurred()
        case .medium: mediumHaptic.impactOccurred()
        }
    }
}
