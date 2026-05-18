import Foundation

/// Vietnamese Telex input method, ported verbatim from the Kotlin
/// reference at DraftRightMobile/android/.../TelexComposer.kt.
/// All algorithm decisions match the Android implementation so the
/// 29 KEYBOARD-MULTI test cases produce identical outputs on iOS.
public final class TelexComposer: Composer {

    private var buffer: String = ""

    public init() {}

    public static let maxLen = 32

    public func onKey(_ char: Character) -> ComposeResult {
        if buffer.count >= TelexComposer.maxLen {
            let committed = buffer
            buffer = String(char)
            return .commit(committed + String(char))
        }

        if !char.isLetter {
            if buffer.isEmpty {
                return .passThrough
            }
            let out = buffer + String(char)
            buffer = ""
            return .commit(out)
        }

        if let combined = TelexComposer.tryCombine(buffer, char) {
            buffer = combined
        } else {
            buffer.append(char)
        }
        return .composing(buffer)
    }

    public func onBackspace() -> ComposeResult {
        if buffer.isEmpty { return .passThrough }
        buffer = TelexComposer.stripOneLayer(buffer)
        return buffer.isEmpty ? .consumed : .composing(buffer)
    }

    public func reset() { buffer = "" }

    public func currentComposingText() -> String { buffer }

    // MARK: - Combining rules

    static func tryCombine(_ buf: String, _ incoming: Character) -> String? {
        if buf.isEmpty { return nil }
        let low = Character(incoming.lowercased())

        // Tone marks (s/f/r/x/j) — apply if buffer contains a vowel-like char.
        if TelexState.isToneMark(low) && buf.contains(where: { TelexState.isVowelLike($0) }) {
            return applyTone(buf, low)
        }

        // 'w' has multiple meanings depending on the preceding chars.
        if low == "w" {
            return applyHornOrBreve(buf, wIsUpper: incoming.isUppercase)
        }

        // dd → đ
        if low == "d" {
            guard let last = buf.last else { return nil }
            if Character(last.lowercased()) == "d" {
                let upper = incoming.isUppercase || last.isUppercase
                let replacement: Character = upper ? "Đ" : "đ"
                return String(buf.dropLast()) + String(replacement)
            }
            return nil
        }

        // Double-vowel circumflex: aa/oo/ee.
        let replacement: Character
        switch low {
        case "a": replacement = "â"
        case "o": replacement = "ô"
        case "e": replacement = "ê"
        default: return nil
        }
        guard let last = buf.last, Character(last.lowercased()) == low else { return nil }
        let upper = incoming.isUppercase || last.isUppercase
        let mapped: Character = upper ? Character(replacement.uppercased()) : replacement
        return String(buf.dropLast()) + String(mapped)
    }

    static func applyHornOrBreve(_ buf: String, wIsUpper: Bool) -> String? {
        // uow special: if buffer ends with "uo" (plain), produce "ươ".
        if buf.count >= 2 {
            let chars = Array(buf)
            let twoBack = chars[chars.count - 2]
            let oneBack = chars[chars.count - 1]
            if Character(twoBack.lowercased()) == "u" && Character(oneBack.lowercased()) == "o" {
                let u2: Character = (twoBack.isUppercase || wIsUpper) ? "Ư" : "ư"
                let o2: Character = (oneBack.isUppercase || wIsUpper) ? "Ơ" : "ơ"
                return String(buf.dropLast(2)) + String(u2) + String(o2)
            }
        }
        guard let last = buf.last else { return nil }
        let replacement: Character
        switch Character(last.lowercased()) {
        case "a": replacement = "ă"
        case "o": replacement = "ơ"
        case "u": replacement = "ư"
        default: return nil
        }
        let mapped: Character = (last.isUppercase || wIsUpper) ? Character(replacement.uppercased()) : replacement
        return String(buf.dropLast()) + String(mapped)
    }

    static func applyTone(_ buf: String, _ toneChar: Character) -> String {
        let chars = Array(buf)
        guard let cluster = findLastVowelCluster(chars) else { return buf }
        let clusterLen = cluster.end - cluster.start + 1
        let hasTrailingConsonant = cluster.end < chars.count - 1

        // Auto-promote 2-vowel ie/uo/ye clusters.
        if clusterLen == 2 && !TelexState.isSpecialVowel(chars[cluster.end]) {
            let first = Character(chars[cluster.start].lowercased())
            let second = Character(chars[cluster.end].lowercased())
            let promoted: Character?
            if (first == "i" && second == "e") || (first == "y" && second == "e") {
                promoted = "ê"
            } else if first == "u" && second == "o" {
                promoted = "ô"
            } else {
                promoted = nil
            }
            if let p = promoted {
                let promotedChar: Character = chars[cluster.end].isUppercase ? Character(p.uppercased()) : p
                var newChars = chars
                newChars[cluster.end] = promotedChar
                return applyToneAt(newChars, cluster.end, toneChar)
            }
        }

        // Auto-promote 3-vowel uoi/ieu/yeu clusters.
        if clusterLen == 3 && !TelexState.isSpecialVowel(chars[cluster.start + 1]) {
            let first = Character(chars[cluster.start].lowercased())
            let mid = Character(chars[cluster.start + 1].lowercased())
            let last = Character(chars[cluster.end].lowercased())
            let promoted: Character?
            if first == "u" && mid == "o" && last == "i" {
                promoted = "ô"
            } else if (first == "i" || first == "y") && mid == "e" && last == "u" {
                promoted = "ê"
            } else {
                promoted = nil
            }
            if let p = promoted {
                let promotedChar: Character = chars[cluster.start + 1].isUppercase ? Character(p.uppercased()) : p
                var newChars = chars
                newChars[cluster.start + 1] = promotedChar
                return applyToneAt(newChars, cluster.start + 1, toneChar)
            }
        }

        let targetIdx = pickToneVowelIndex(chars, cluster.start, cluster.end, hasTrailingConsonant)
        return applyToneAt(chars, targetIdx, toneChar)
    }

    private struct ClusterRange { let start: Int; let end: Int }

    private static func findLastVowelCluster(_ chars: [Character]) -> ClusterRange? {
        var end = chars.count
        while end > 0 && !TelexState.isVowelLike(chars[end - 1]) { end -= 1 }
        let clusterEndExclusive = end
        while end > 0 && TelexState.isVowelLike(chars[end - 1]) { end -= 1 }
        let clusterStart = end
        guard clusterStart < clusterEndExclusive else { return nil }
        return ClusterRange(start: clusterStart, end: clusterEndExclusive - 1)
    }

    private static func pickToneVowelIndex(
        _ chars: [Character],
        _ start: Int,
        _ endInclusive: Int,
        _ hasTrailingConsonant: Bool
    ) -> Int {
        let len = endInclusive - start + 1
        if len >= 3 { return start + 1 }
        if len == 2 {
            let first = chars[start]
            let second = chars[endInclusive]
            if TelexState.isSpecialVowel(second) { return endInclusive }
            if TelexState.isSpecialVowel(first) { return start }
            return hasTrailingConsonant ? endInclusive : start
        }
        return start
    }

    private static func applyToneAt(_ chars: [Character], _ idx: Int, _ toneChar: Character) -> String {
        let baseChar = chars[idx]
        guard let toned = applyToneToChar(baseChar, toneChar) else {
            return String(chars)
        }
        var newChars = chars
        newChars[idx] = toned
        return String(newChars)
    }

    private static func applyToneToChar(_ base: Character, _ tone: Character) -> Character? {
        guard let toneIdx = toneIndex[tone] else { return nil }
        let baseLower = Character(base.lowercased())
        let baseRoot = unTone[baseLower] ?? baseLower
        guard let row = toneRowsLower[baseRoot] else { return nil }
        let toned = row[toneIdx]
        return base.isUppercase ? Character(toned.uppercased()) : toned
    }

    static func stripOneLayer(_ buf: String) -> String {
        if buf.isEmpty { return "" }
        let last = buf.last!
        let rest = String(buf.dropLast())
        let isUpper = last.isUppercase

        let lower = Character(last.lowercased())

        // 1. Strip tone if last char is toned.
        if let untoned = unTone[lower] {
            let mapped: Character = isUpper ? Character(untoned.uppercased()) : untoned
            return rest + String(mapped)
        }
        // 2. Strip diacritic mark (ă/â/ê/ô/ơ/ư/đ → base).
        if let unmarked = unMark[lower] {
            let mapped: Character = isUpper ? Character(unmarked.uppercased()) : unmarked
            return rest + String(mapped)
        }
        // 3. Drop the char.
        return rest
    }

    // MARK: - Tone tables (matches Kotlin source)

    private static let toneIndex: [Character: Int] = [
        "s": 0, "f": 1, "r": 2, "x": 3, "j": 4,
    ]

    // Order: acute, grave, hook, tilde, dot-below.
    private static let toneRowsLower: [Character: [Character]] = [
        "a": ["á", "à", "ả", "ã", "ạ"],
        "ă": ["ắ", "ằ", "ẳ", "ẵ", "ặ"],
        "â": ["ấ", "ầ", "ẩ", "ẫ", "ậ"],
        "e": ["é", "è", "ẻ", "ẽ", "ẹ"],
        "ê": ["ế", "ề", "ể", "ễ", "ệ"],
        "i": ["í", "ì", "ỉ", "ĩ", "ị"],
        "o": ["ó", "ò", "ỏ", "õ", "ọ"],
        "ô": ["ố", "ồ", "ổ", "ỗ", "ộ"],
        "ơ": ["ớ", "ờ", "ở", "ỡ", "ợ"],
        "u": ["ú", "ù", "ủ", "ũ", "ụ"],
        "ư": ["ứ", "ừ", "ử", "ữ", "ự"],
        "y": ["ý", "ỳ", "ỷ", "ỹ", "ỵ"],
    ]

    // Reverse map: toned → untoned root.
    private static let unTone: [Character: Character] = {
        var dict: [Character: Character] = [:]
        for (base, row) in toneRowsLower {
            for toned in row {
                dict[toned] = base
            }
        }
        return dict
    }()

    // Mark removal: special vowels and đ → bare ASCII root.
    private static let unMark: [Character: Character] = [
        "ă": "a", "â": "a",
        "ê": "e",
        "ô": "o", "ơ": "o",
        "ư": "u",
        "đ": "d",
    ]
}
