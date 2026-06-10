import Foundation

/// Loads a TSV word list (and optional bigram file) into an
/// `InMemoryWordList`. Two-column word lines (`word<TAB>frequency`);
/// three-column bigram lines (`prev<TAB>next<TAB>count`). Lines starting
/// with `#` are comments; blank lines are skipped.
///
/// Mirror of `keyboard.ime.WordListLoader` on Android — same format so
/// the same `wordlist_vi.tsv` ships unchanged for both platforms.
public enum WordListLoader {

    /// Load a word list bundled inside an XCFramework / SwiftPM resource
    /// or living on disk inside the keyboard extension's App Group
    /// container (where downloaded packs land in production).
    public static func loadWords(from url: URL, bigrams bigramsURL: URL? = nil) throws -> InMemoryWordList {
        let words = try parseWords(url: url)
        let bigrams = try bigramsURL.map { try parseBigrams(url: $0) } ?? [:]
        return InMemoryWordList(words: words, bigrams: bigrams)
    }

    private static func parseWords(url: URL) throws -> [(word: String, freq: Int)] {
        let raw = try String(contentsOf: url, encoding: .utf8)
        var out: [(String, Int)] = []
        out.reserveCapacity(2048)
        for rawLine in raw.split(separator: "\n", omittingEmptySubsequences: true) {
            let line = rawLine.trimmingCharacters(in: .whitespaces)
            if line.isEmpty || line.hasPrefix("#") { continue }
            let tab = line.firstIndex(of: "\t") ?? line.endIndex
            guard tab != line.startIndex, tab != line.endIndex else { continue }
            let word = line[..<tab].trimmingCharacters(in: .whitespaces)
            let freqStr = line[line.index(after: tab)...].trimmingCharacters(in: .whitespaces)
            guard !word.isEmpty, let freq = Int(freqStr) else { continue }
            out.append((word, freq))
        }
        return out
    }

    private static func parseBigrams(url: URL) throws -> [String: [String: Int]] {
        let raw = try String(contentsOf: url, encoding: .utf8)
        var out: [String: [String: Int]] = [:]
        for rawLine in raw.split(separator: "\n", omittingEmptySubsequences: true) {
            let line = rawLine.trimmingCharacters(in: .whitespaces)
            if line.isEmpty || line.hasPrefix("#") { continue }
            let parts = line.split(separator: "\t", omittingEmptySubsequences: false).map(String.init)
            guard parts.count == 3 else { continue }
            let prev = parts[0].trimmingCharacters(in: .whitespaces)
            let next = parts[1].trimmingCharacters(in: .whitespaces)
            guard !prev.isEmpty, !next.isEmpty, let count = Int(parts[2].trimmingCharacters(in: .whitespaces)) else { continue }
            out[prev, default: [:]][next, default: 0] += count
        }
        return out
    }
}
