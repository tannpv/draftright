import Foundation

/// Picks the most-recent installed JP dictionary pack for a language and
/// falls back to the bundled seed when none is installed.
///
/// Mirrors `WordListPackResolver` but typed to `[String: [String]]`
/// (reading → kanji candidates) instead of `LanguageWordList`, so each
/// engine type has a clean, dedicated resolver (Rule #1 — no stringly-typed
/// generics or force-casts).
///
/// Pack files live under `<App Group container>/packs/<prefix>-v<N>.pack`,
/// written by the Flutter `ImePackService` in the host app.
public enum DictPackResolver {

    /// - Parameters:
    ///   - appGroupContainer: URL from `FileManager.containerURL(forSecurityApplicationGroupIdentifier:)`,
    ///                        or nil to skip the installed-pack lookup (sim / unit tests).
    ///   - packIdPrefix: Stable prefix matching the backend manifest URL
    ///                   (e.g. `"draftright-ime-ja"`).
    ///   - fallback: Closure returning the built-in seed dictionary when no
    ///               installed pack is found.
    public static func loadOrFallback(
        appGroupContainer: URL?,
        packIdPrefix: String,
        fallback: () -> [String: [String]]
    ) -> [String: [String]] {
        if let installed = findLatestInstalled(appGroupContainer: appGroupContainer, packIdPrefix: packIdPrefix) {
            if let dict = try? DictPackLoader.load(from: installed), !dict.isEmpty {
                return dict
            }
        }
        return fallback()
    }

    private static func findLatestInstalled(appGroupContainer: URL?, packIdPrefix: String) -> URL? {
        guard let container = appGroupContainer else { return nil }
        let packsDir = container.appendingPathComponent("packs", isDirectory: true)
        let fm = FileManager.default
        guard let entries = try? fm.contentsOfDirectory(at: packsDir, includingPropertiesForKeys: nil) else { return nil }
        let pattern = "^\(NSRegularExpression.escapedPattern(for: packIdPrefix))-v(\\d+)\\.pack$"
        guard let regex = try? NSRegularExpression(pattern: pattern) else { return nil }
        var bestVersion = -1
        var bestURL: URL?
        for entry in entries {
            let name = entry.lastPathComponent
            let range = NSRange(name.startIndex..<name.endIndex, in: name)
            guard let match = regex.firstMatch(in: name, range: range),
                  match.numberOfRanges == 2,
                  let versionRange = Range(match.range(at: 1), in: name),
                  let version = Int(name[versionRange]) else { continue }
            let size = (try? fm.attributesOfItem(atPath: entry.path)[.size] as? Int) ?? 0
            if size > 0, version > bestVersion {
                bestVersion = version
                bestURL = entry
            }
        }
        return bestURL
    }
}
