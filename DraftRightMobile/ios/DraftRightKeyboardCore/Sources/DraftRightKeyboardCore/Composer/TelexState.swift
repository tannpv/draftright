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
    /// Vowels that can sit AFTER the nucleus as an offglide (semivowel coda) —
    /// the trailing i/y/u/o of "day", "tai", "keo", "cau". A modifier typed at
    /// the end may reach back over these to the nucleus, but not over another
    /// full nucleus vowel. Mirrors Android's TelexState.GLIDE_CODAS.
    static let glideCodas: Set<Character> = [
        "i", "y", "u", "o",
        "I", "Y", "U", "O",
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

    public static func isGlideCoda(_ c: Character) -> Bool {
        glideCodas.contains(c)
    }

    public static func isToneMark(_ c: Character) -> Bool {
        toneMarks.contains(c)
    }
}
