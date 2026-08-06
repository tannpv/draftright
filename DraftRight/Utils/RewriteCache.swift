import Foundation

/// In-memory cache for rewritten text. Key = (language, tone, text).
///
/// Avoids a redundant backend round-trip when the user retries the same
/// text+tone combo — reopening the panel, or toggling tones back and forth.
///
/// Kept at parity with `DraftRightWindows/.../Services/RewriteCache.cs` and
/// `DraftRightLinux/draftright/services/rewrite_cache.py`; the three produce
/// identical keys and identical eviction, so behaviour and hit rates do not
/// differ per platform (#147).
final class RewriteCache {
    static let shared = RewriteCache()

    /// Entries held before eviction kicks in.
    static let defaultMaxEntries = 200

    /// Evict roughly 1/N of the cache in one pass when it fills up (N = 4 →
    /// 25%), rather than trimming a single entry on every insert once full.
    static let defaultEvictFraction = 4

    private let maxEntries: Int
    private let evictFraction: Int
    private var cache: [String: String] = [:]

    /// Insertion order, for real FIFO eviction.
    ///
    /// The previous implementation evicted `cache.keys.prefix(maxEntries / 4)`.
    /// Swift dictionaries have NO defined key order, so that dropped an
    /// arbitrary quarter while the comment claimed it dropped the oldest.
    private var order: [String] = []

    private let lock = NSLock()

    /// `init` is internal rather than private so tests can build a small cache
    /// and exercise eviction without inserting 200 entries.
    init(maxEntries: Int = RewriteCache.defaultMaxEntries,
         evictFraction: Int = RewriteCache.defaultEvictFraction) {
        self.maxEntries = max(1, maxEntries)
        self.evictFraction = max(1, evictFraction)
    }

    /// Cache key. The target language is prepended when it affects the result;
    /// without it, translating to one language and then switching would serve
    /// the previous language's translation — wrong output, not just a miss.
    static func key(text: String, tone: String, language: String? = nil) -> String {
        let basis = "\(tone)::\(text)"
        guard let language, !language.isEmpty else { return basis }
        return "\(language)::\(basis)"
    }

    func get(text: String, tone: String, language: String? = nil) -> String? {
        lock.lock(); defer { lock.unlock() }
        return cache[Self.key(text: text, tone: tone, language: language)]
    }

    func set(text: String, tone: String, result: String, language: String? = nil) {
        let k = Self.key(text: text, tone: tone, language: language)
        lock.lock(); defer { lock.unlock() }

        if cache[k] == nil && cache.count >= maxEntries {
            let remove = max(1, maxEntries / evictFraction)
            for key in order.prefix(remove) {
                cache.removeValue(forKey: key)
            }
            order.removeFirst(min(remove, order.count))
        }
        // Only record insertion order on first insert, so re-setting an
        // existing key updates the value without changing its position.
        if cache[k] == nil { order.append(k) }
        cache[k] = result
    }

    /// Drop everything — called on sign-out so one account's rewrites are
    /// never served to whoever signs in next.
    func clear() {
        lock.lock(); defer { lock.unlock() }
        cache.removeAll()
        order.removeAll()
    }

    var count: Int {
        lock.lock(); defer { lock.unlock() }
        return cache.count
    }
}
