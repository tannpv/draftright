import CoreGraphics

/// Pure geometry for the hold-to-talk mic gesture. Kept UIKit-free so it is
/// unit-testable without touch events. The view feeds displacement from the
/// touch-down point; this decides whether the finger has slid far enough to
/// mean "cancel" (slide-away-to-cancel, Zalo/WhatsApp style). Mirrors the
/// Android `MicHoldGesture`.
public enum MicHoldGesture {
    /// Delay before a hold starts recording; a shorter press is a no-op tap.
    public static let armMs = 180

    /// Default slide-to-cancel threshold, in points.
    public static let defaultSlop: CGFloat = 32

    /// True once the finger has slid `slop` or more up or left from the down
    /// point. Screen y grows downward, so "up" is a negative dy and "left" is a
    /// negative dx. Deliberately NOT omnidirectional: a finger relaxing or the
    /// phone tilting down while speaking is common and shouldn't silently
    /// discard a good dictation, so downward/rightward drift never arms cancel.
    public static func isCancelArmed(dx: CGFloat, dy: CGFloat, slop: CGFloat) -> Bool {
        dy <= -slop || dx <= -slop
    }
}
