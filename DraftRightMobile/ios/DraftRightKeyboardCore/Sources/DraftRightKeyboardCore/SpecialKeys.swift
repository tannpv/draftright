import Foundation

public enum SpecialKeys {
    public static let shift = -1
    public static let symbols = -2
    public static let globe = -3
    public static let enter = -4
    public static let backspace = -5
    public static let symbols2 = -6
    public static let alpha = -7
    public static let globePicker = -8

    public static func isSpecial(_ code: Int) -> Bool { code < 0 }
}
