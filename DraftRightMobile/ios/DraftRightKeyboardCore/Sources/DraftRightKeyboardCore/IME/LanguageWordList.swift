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

    func close()
}

public extension LanguageWordList {
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
}
