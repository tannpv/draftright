/// The three states of the keyboard shift key, mirroring the stock Samsung
/// keyboard: off (lowercase), single (capitalize the next character only, then
/// auto-revert), capsLock (capitalize every character until toggled off).
///
/// Lives in Core so the shift cycle and auto-capitalization policy are
/// unit-testable without UIKit.
public enum ShiftState {
    case off, single, capsLock
}

public extension ShiftState {
    /// Advance on a single tap, matching the stock Samsung keyboard: one button
    /// cycles off -> single -> capsLock -> off. No double-tap.
    func nextOnTap() -> ShiftState {
        switch self {
        case .off: return .single
        case .single: return .capsLock
        case .capsLock: return .off
        }
    }

    /// SF Symbol name for each state, giving the three states a distinct look:
    /// outline shift / filled shift / caps-lock.
    var iconName: String {
        switch self {
        case .off: return "shift"
        case .single: return "shift.fill"
        case .capsLock: return "capslock.fill"
        }
    }
}
