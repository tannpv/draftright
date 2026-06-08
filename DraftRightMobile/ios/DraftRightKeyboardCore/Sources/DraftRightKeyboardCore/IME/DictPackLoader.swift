import Foundation

/// Parses a Japanese reading→kanji dictionary pack (`.pack`) into the map
/// that `JapaneseDictionaryEngine` consumes.
///
/// Format — one reading per line:
///   `reading<TAB>kanji1,kanji2,...`
/// Lines starting with `#` are comments; blank lines are skipped.
/// Rank order of candidates is preserved (first = most preferred).
///
/// Rule #1: pure parsing, no platform I/O, no hardcoded filenames — callers
/// supply the URL; `JapaneseLanguagePack` wires the resolver.
public enum DictPackLoader {

    /// Load a reading→kanji map from a `.pack` URL on disk.
    public static func load(from url: URL) throws -> [String: [String]] {
        let raw = try String(contentsOf: url, encoding: .utf8)
        return parse(raw)
    }

    /// Parse raw TSV content — useful for testing without disk I/O.
    public static func parse(_ tsv: String) -> [String: [String]] {
        var result: [String: [String]] = [:]
        for line in tsv.split(separator: "\n", omittingEmptySubsequences: true) {
            let s = line.trimmingCharacters(in: .whitespaces)
            if s.isEmpty || s.hasPrefix("#") { continue }
            guard let tabIdx = s.firstIndex(of: "\t") else { continue }
            let reading = String(s[..<tabIdx]).trimmingCharacters(in: .whitespaces)
            let candidatesRaw = String(s[s.index(after: tabIdx)...]).trimmingCharacters(in: .whitespaces)
            if reading.isEmpty || candidatesRaw.isEmpty { continue }
            let candidates = candidatesRaw
                .split(separator: ",", omittingEmptySubsequences: true)
                .map { $0.trimmingCharacters(in: .whitespaces) }
                .filter { !$0.isEmpty }
            if !candidates.isEmpty { result[reading] = candidates }
        }
        return result
    }
}
