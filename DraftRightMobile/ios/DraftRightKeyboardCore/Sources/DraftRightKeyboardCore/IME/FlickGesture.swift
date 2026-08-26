import Foundation

/// The five flick outcomes on a 12-key Japanese kana key (#212).
public enum FlickDirection: Hashable { case tap, left, up, right, down }

/// Resolves a touch movement into a flick direction (#212). Pure math (Double,
/// no UIKit) so it unit-tests directly and mirrors the Kotlin `FlickGesture`.
/// Screen coordinates: y increases downward, so an upward flick has negative dy.
public enum FlickGesture {

    public static func resolve(dx: Double, dy: Double, tapThreshold: Double) -> FlickDirection {
        if abs(dx) < tapThreshold && abs(dy) < tapThreshold { return .tap }
        if abs(dx) >= abs(dy) { return dx < 0 ? .left : .right }
        return dy < 0 ? .up : .down
    }
}
