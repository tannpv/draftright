import Foundation

/// Picks the most-recent installed wordlist pack for a language and falls
/// back to the bundled bootstrap when none is installed.
///
/// Mirror of `keyboard.ime.WordListPackResolver` on Android. Packs live
/// under `<App Group container>/packs/<prefix>-v<N>.pack`, written by the
/// Flutter `ImePackService` in the host app and read here by the iOS
/// keyboard extension via App Group sharing.
///
/// Per Rule #1: this is the single point that swaps "downloaded" for
/// "bundled" — `VietnameseLanguagePack` (and any future Latin pack) just
/// calls `loadOrFallback` and gets a `LanguageWordList` regardless of
/// which source served it.
public enum WordListPackResolver {

    /// - Parameters:
    ///   - appGroupContainer: URL returned by
    ///     `FileManager.containerURL(forSecurityApplicationGroupIdentifier:)`,
    ///     or nil to skip the installed-pack lookup entirely.
    ///   - packIdPrefix: Stable prefix matching the backend manifest's
    ///                   download URL (e.g. `"draftright-wordlist-vi"`).
    ///   - fallback: Closure that returns the bundled `LanguageWordList`
    ///               when no installed pack is found.
    public static func loadOrFallback(
        appGroupContainer: URL?,
        packIdPrefix: String,
        fallback: () -> LanguageWordList
    ) -> LanguageWordList {
        if let installed = findLatestInstalled(appGroupContainer: appGroupContainer, packIdPrefix: packIdPrefix) {
            do {
                return try WordListLoader.loadWords(from: installed)
            } catch {
                // Corrupt or unreadable pack — don't kill suggestions.
                // The caller can still log via its own pipeline.
            }
        }
        return fallback()
    }

    /// Returns the highest-version installed pack URL matching the prefix,
    /// or nil when nothing is installed. Versioning lets a rollout cleanly
    /// coexist with an older client mid-update.
    private static func findLatestInstalled(appGroupContainer: URL?, packIdPrefix: String) -> URL? {
        guard let container = appGroupContainer else { return nil }
        let packsDir = container.appendingPathComponent("packs", isDirectory: true)
        let fm = FileManager.default
        guard let entries = try? fm.contentsOfDirectory(at: packsDir, includingPropertiesForKeys: nil) else {
            return nil
        }
        // Pattern: "<prefix>-v<digits>.pack"
        let pattern = "^\(NSRegularExpression.escapedPattern(for: packIdPrefix))-v(\\d+)\\.pack$"
        guard let regex = try? NSRegularExpression(pattern: pattern) else { return nil }
        var bestVersion = -1
        var bestURL: URL? = nil
        for entry in entries {
            let name = entry.lastPathComponent
            let range = NSRange(name.startIndex..<name.endIndex, in: name)
            guard let match = regex.firstMatch(in: name, range: range) else { continue }
            guard match.numberOfRanges == 2,
                  let versionRange = Range(match.range(at: 1), in: name),
                  let version = Int(name[versionRange]) else { continue }
            // Skip empty files (failed download stubs).
            let size = (try? fm.attributesOfItem(atPath: entry.path)[.size] as? Int) ?? 0
            if size > 0 && version > bestVersion {
                bestVersion = version
                bestURL = entry
            }
        }
        return bestURL
    }
}
