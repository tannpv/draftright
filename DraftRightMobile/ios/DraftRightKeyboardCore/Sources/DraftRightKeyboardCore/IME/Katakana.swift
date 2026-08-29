import Foundation

/// Hiragana → katakana transliteration (#211/#212): every JP IME offers the
/// katakana form of the typed reading as a candidate (かな → カナ), which is how
/// users write loanwords (コーヒー, パソコン) and names. The hiragana block
/// U+3041..U+3096 maps 1:1 to katakana U+30A1..U+30F6 by a fixed +0x60 offset
/// (Unicode standard); ー (chōonpu), 〜 and anything outside the block pass
/// through unchanged. Pure function, mirror of the Kotlin `Katakana`.
public enum Katakana {
    private static let hiraStart: UInt32 = 0x3041
    private static let hiraEnd: UInt32 = 0x3096
    private static let toKatakana: UInt32 = 0x60

    public static func fromHiragana(_ reading: String) -> String {
        var out = String.UnicodeScalarView()
        for scalar in reading.unicodeScalars {
            if scalar.value >= hiraStart && scalar.value <= hiraEnd,
               let shifted = Unicode.Scalar(scalar.value + toKatakana) {
                out.append(shifted)
            } else {
                out.append(scalar)
            }
        }
        return String(out)
    }
}
