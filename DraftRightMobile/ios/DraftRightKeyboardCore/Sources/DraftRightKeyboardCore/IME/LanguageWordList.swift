import Foundation

/// Read-only view of a language pack's word frequency table + bigram
/// successor map. Implementations vary by storage:
///   - `InMemoryWordList`  — small built-in lists shipped with the app
///                           (bootstrap path while the real pack downloads).
///   - `MmapWordList`      — mmap'd binary table from a downloaded pack
///                           (production path; sub-millisecond lookups
///                           even for 50k words).
///
/// Mirror of `keyboard.ime.LanguageWordList` on Android. The interface
/// stays narrow so the engine doesn't care which storage it's reading
/// from. Swapping mmap in later won't touch `TrigramCandidateEngine`.
public protocol LanguageWordList {
    /// Words whose lowercase form starts with `prefix`, paired with their
    /// frequency (higher = more common). Up to `limit` entries, in any
    /// order. Original casing is preserved so a proper-noun list ("Saigon")
    /// doesn't get lowercased on render.
    func prefixMatches(_ prefix: String, limit: Int) -> [(word: String, freq: Int)]

    /// Successor words seen after `token` in the training corpus, mapped
    /// to a co-occurrence count. Empty when `token` is unknown.
    func successors(_ token: String) -> [String: Int]

    /// Dictionary words within `maxEdits` Levenshtein distance of `term`, for
    /// typo tolerance / autocorrect (#207). Mirror of Android
    /// `LanguageWordList.fuzzyMatches`. Default empty — an mmap store would need
    /// its own index rather than scanning 50k rows, so it opts in.
    func fuzzyMatches(_ term: String, maxEdits: Int, limit: Int) -> [(word: String, freq: Int)]

    func close()
}

public extension LanguageWordList {
    func fuzzyMatches(_ term: String, maxEdits: Int, limit: Int) -> [(word: String, freq: Int)] { [] }
    func close() {}
}

/// Tiny in-memory implementation. Sufficient for tests + the bootstrap
/// path; production swaps an `MmapWordList`.
public final class InMemoryWordList: LanguageWordList {
    private let words: [(word: String, freq: Int)]
    private let lowerBigrams: [String: [String: Int]]

    /// - Parameters:
    ///   - words:   Word/frequency pairs. Sorted by descending frequency
    ///              by the initializer so prefix scans hit the most-likely
    ///              candidates first.
    ///   - bigrams: Successor map keyed by the *preceding* word. Keys are
    ///              lowercased on construction so lookups don't have to
    ///              think about the source casing.
    public init(words: [(word: String, freq: Int)], bigrams: [String: [String: Int]] = [:]) {
        self.words = words.sorted { $0.freq > $1.freq }
        var lc: [String: [String: Int]] = [:]
        lc.reserveCapacity(bigrams.count)
        for (k, v) in bigrams { lc[k.lowercased()] = v }
        self.lowerBigrams = lc
    }

    public func prefixMatches(_ prefix: String, limit: Int) -> [(word: String, freq: Int)] {
        if prefix.isEmpty || limit <= 0 { return [] }
        let lc = prefix.lowercased()
        var out: [(String, Int)] = []
        out.reserveCapacity(min(limit, words.count))
        for entry in words {
            if entry.word.lowercased().hasPrefix(lc) {
                out.append((entry.word, entry.freq))
                if out.count >= limit { break }
            }
        }
        return out
    }

    public func successors(_ token: String) -> [String: Int] {
        lowerBigrams[token.lowercased()] ?? [:]
    }

    /// Full scan for words within `maxEdits` of `term`, ranked by (distance asc,
    /// frequency desc). Cheap for the bootstrap list; the mmap store overrides
    /// with an index. Compares lowercased so casing isn't counted as an edit.
    /// Mirrors Android `InMemoryWordList.fuzzyMatches`.
    public func fuzzyMatches(_ term: String, maxEdits: Int, limit: Int) -> [(word: String, freq: Int)] {
        if term.isEmpty || limit <= 0 || maxEdits <= 0 { return [] }
        let a = Array(term.lowercased())
        var hits: [(word: String, freq: Int, dist: Int)] = []
        for entry in words {
            let d = Self.boundedLevenshtein(a, Array(entry.word.lowercased()), max: maxEdits)
            if d >= 1 && d <= maxEdits { hits.append((entry.word, entry.freq, d)) }
        }
        hits.sort { $0.dist != $1.dist ? $0.dist < $1.dist : $0.freq > $1.freq }
        return hits.prefix(limit).map { ($0.word, $0.freq) }
    }

    /// Levenshtein distance, short-circuiting to `max`+1 once every cell in a
    /// row exceeds `max`. Mirrors Android `boundedLevenshtein`.
    private static func boundedLevenshtein(_ a: [Character], _ b: [Character], max: Int) -> Int {
        if abs(a.count - b.count) > max { return max + 1 }
        if a.isEmpty { return b.count }
        if b.isEmpty { return a.count }
        var prev = Array(0...b.count)
        var curr = [Int](repeating: 0, count: b.count + 1)
        for i in 1...a.count {
            curr[0] = i
            var rowMin = curr[0]
            for j in 1...b.count {
                let cost = a[i - 1] == b[j - 1] ? 0 : 1
                curr[j] = Swift.min(prev[j] + 1, curr[j - 1] + 1, prev[j - 1] + cost)
                if curr[j] < rowMin { rowMin = curr[j] }
            }
            if rowMin > max { return max + 1 }
            swap(&prev, &curr)
        }
        return prev[b.count]
    }
}
