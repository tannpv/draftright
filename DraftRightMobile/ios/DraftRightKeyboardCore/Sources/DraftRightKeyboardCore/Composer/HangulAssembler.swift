import Foundation

/// Assembles Hangul compatibility jamo (ㄱ, ㅏ, …) into syllable blocks (가, 한, …).
/// Deterministic composition — the whole of Korean input, no dictionary.
/// Pure function over the full jamo string (fits BufferingComposer.transform).
/// Mirrors the Kotlin HangulAssembler.
enum HangulAssembler {

    private static let cho = Array("ㄱㄲㄴㄷㄸㄹㅁㅂㅃㅅㅆㅇㅈㅉㅊㅋㅌㅍㅎ")
    private static let jung = Array("ㅏㅐㅑㅒㅓㅔㅕㅖㅗㅘㅙㅚㅛㅜㅝㅞㅟㅠㅡㅢㅣ")
    private static let jong = Array(" ㄱㄲㄳㄴㄵㄶㄷㄹㄺㄻㄼㄽㄾㄿㅀㅁㅂㅄㅅㅆㅇㅈㅊㅋㅌㅍㅎ") // [0]=none

    private static let compoundJung: [String: Character] = [
        "ㅗㅏ": "ㅘ", "ㅗㅐ": "ㅙ", "ㅗㅣ": "ㅚ",
        "ㅜㅓ": "ㅝ", "ㅜㅔ": "ㅞ", "ㅜㅣ": "ㅟ", "ㅡㅣ": "ㅢ",
    ]
    private static let compoundJong: [String: Character] = [
        "ㄱㅅ": "ㄳ", "ㄴㅈ": "ㄵ", "ㄴㅎ": "ㄶ", "ㄹㄱ": "ㄺ", "ㄹㅁ": "ㄻ",
        "ㄹㅂ": "ㄼ", "ㄹㅅ": "ㄽ", "ㄹㅌ": "ㄾ", "ㄹㅍ": "ㄿ", "ㄹㅎ": "ㅀ", "ㅂㅅ": "ㅄ",
    ]
    private static let splitJong: [Character: (Character, Character)] = {
        var m: [Character: (Character, Character)] = [:]
        for (k, v) in compoundJong { let a = Array(k); m[v] = (a[0], a[1]) }
        return m
    }()

    private static func isCho(_ c: Character) -> Bool { cho.contains(c) }
    private static func isJung(_ c: Character) -> Bool { jung.contains(c) }
    private static func isValidJong(_ c: Character) -> Bool { (jong.firstIndex(of: c) ?? 0) > 0 }

    private static func compose(_ c: Character?, _ j: Character?, _ k: Character?) -> String {
        if let c = c, let j = j, let ci = cho.firstIndex(of: c), let ji = jung.firstIndex(of: j) {
            let ki = k != nil ? (jong.firstIndex(of: k!) ?? 0) : 0
            let code = 0xAC00 + (ci * 21 + ji) * 28 + ki
            return String(UnicodeScalar(code)!)
        }
        var s = ""
        if let c = c { s.append(c) }
        if let j = j { s.append(j) }
        if let k = k { s.append(k) }
        return s
    }

    static func assemble(_ jamo: String) -> String {
        var out = ""
        var c: Character?
        var j: Character?
        var k: Character?

        func flush() { out += compose(c, j, k); c = nil; j = nil; k = nil }

        for ch in jamo {
            if isCho(ch) || isValidJong(ch) {
                if c == nil && j == nil {
                    c = ch
                } else if c != nil && j == nil {
                    flush(); c = ch
                } else if j != nil && k == nil {
                    if isValidJong(ch) { k = ch } else { flush(); c = ch }
                } else {
                    if let comp = compoundJong[String(k!) + String(ch)] { k = comp }
                    else { flush(); c = ch }
                }
            } else if isJung(ch) {
                if c == nil && j == nil {
                    j = ch
                } else if j == nil {
                    j = ch
                } else if k == nil {
                    if let comp = compoundJung[String(j!) + String(ch)] { j = comp }
                    else { flush(); j = ch }
                } else {
                    if let split = splitJong[k!] {
                        k = split.0
                        let moved = split.1
                        flush(); c = moved; j = ch
                    } else {
                        let moved = k
                        k = nil
                        flush(); c = moved; j = ch
                    }
                }
            } else {
                flush(); out.append(ch)
            }
        }
        out += compose(c, j, k)
        return out
    }
}
