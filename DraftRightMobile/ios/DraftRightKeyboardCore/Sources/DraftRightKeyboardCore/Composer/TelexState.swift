import Foundation

public enum TelexState {
    static let plainVowels: Set<Character> = [
        "a", "e", "i", "o", "u", "y",
        "A", "E", "I", "O", "U", "Y",
    ]
    static let specialVowels: Set<Character> = [
        "ă", "â", "ê", "ô", "ơ", "ư",
        "Ă", "Â", "Ê", "Ô", "Ơ", "Ư",
    ]
    static let toneMarks: Set<Character> = ["s", "f", "r", "x", "j"]

    public static func isVowel(_ c: Character) -> Bool {
        plainVowels.contains(c)
    }

    public static func isVowelLike(_ c: Character) -> Bool {
        plainVowels.contains(c) || specialVowels.contains(c)
    }

    public static func isSpecialVowel(_ c: Character) -> Bool {
        specialVowels.contains(c)
    }

    public static func isToneMark(_ c: Character) -> Bool {
        toneMarks.contains(c)
    }
}
