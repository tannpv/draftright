import Foundation

/// In-memory cache for rewritten text. Key = original text + tone.
/// Avoids redundant OpenAI calls when user retries the same text/tone combo.
final class RewriteCache {
    static let shared = RewriteCache()

    private var cache: [String: String] = [:]
    private let maxEntries = 200

    private init() {}

    private func key(text: String, tone: String) -> String {
        return "\(tone)::\(text)"
    }

    func get(text: String, tone: String) -> String? {
        return cache[key(text: text, tone: tone)]
    }

    func set(text: String, tone: String, result: String) {
        // Evict oldest entries if cache is full
        if cache.count >= maxEntries {
            // Remove ~25% of entries
            let keysToRemove = Array(cache.keys.prefix(maxEntries / 4))
            for k in keysToRemove {
                cache.removeValue(forKey: k)
            }
        }
        cache[key(text: text, tone: tone)] = result
    }

    func clear() {
        cache.removeAll()
    }
}
