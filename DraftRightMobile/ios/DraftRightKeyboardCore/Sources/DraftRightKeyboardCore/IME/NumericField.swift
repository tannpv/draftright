/// Whether the keyboard should open on the digits layer for a field with this
/// `UIKeyboardType` (#190) — e.g. OTP, PIN, phone, amount fields.
///
/// Takes the raw value, not `UIKeyboardType`, so the rule lives in the pure
/// Core and unit-tests without UIKit. The values are the stable
/// `UIKeyboardType` ABI, named locally so the one place that knows "which
/// keyboard types count as numeric" is here (RULE #1) — the Android side
/// mirrors it with the equivalent `EditorInfo.inputType` classes.
public enum NumericField {
    // UIKeyboardType raw values for the digits-only pads.
    private static let numberPad = 4
    private static let phonePad = 5
    private static let decimalPad = 8
    private static let asciiCapableNumberPad = 11

    public static func isNumericKeyboard(_ rawValue: Int) -> Bool {
        switch rawValue {
        case numberPad, phonePad, decimalPad, asciiCapableNumberPad:
            return true
        default:
            return false
        }
    }
}
