package com.draftright.keyboard.ime

/**
 * Read-only view of a language pack's word frequency table + bigram
 * successor map. Implementations vary by storage:
 *   - InMemoryWordList — small built-in lists shipped with the APK (the
 *     bootstrap path while the real pack downloads).
 *   - MmapWordList    — mmap'd binary table from a downloaded pack
 *     (the production path; sub-millisecond lookups even for 50k words).
 *
 * The interface is intentionally narrow so the engine doesn't care which
 * storage it's reading from. Per Rule #1, swapping mmap in later won't
 * touch TrigramCandidateEngine.
 */
interface LanguageWordList {
    /**
     * Words whose lowercase form starts with [prefix], paired with their
     * frequency (higher = more common). Up to [limit] entries, in any order.
     * Case of the original word is preserved in the returned key so a
     * proper-noun list ("Saigon") doesn't get lowercased on render.
     */
    fun prefixMatches(prefix: String, limit: Int): List<Pair<String, Int>>

    /**
     * Successor words seen after [token] in the training corpus, mapped to
     * a co-occurrence count. Empty map when [token] is unknown.
     */
    fun successors(token: String): Map<String, Int>

    fun close() {}
}

/**
 * Tiny in-memory implementation. Sufficient for tests + the bootstrap
 * path; production swaps an MmapWordList implementation.
 */
class InMemoryWordList(
    private val words: List<Pair<String, Int>>,
    bigrams: Map<String, Map<String, Int>> = emptyMap(),
) : LanguageWordList {

    /**
     * Bigrams keyed by lowercased preceding word so callers don't have to
     * worry about source casing. Bigram entries are independent of the
     * unigram word list — a context word can predict successors even when
     * the context itself isn't a completable target.
     */
    private val lcBigrams: Map<String, Map<String, Int>> =
        bigrams.mapKeys { (k, _) -> k.lowercase() }

    override fun prefixMatches(prefix: String, limit: Int): List<Pair<String, Int>> {
        if (prefix.isEmpty() || limit <= 0) return emptyList()
        val lc = prefix.lowercase()
        val out = ArrayList<Pair<String, Int>>(limit.coerceAtMost(words.size))
        for ((word, freq) in words) {
            if (word.lowercase().startsWith(lc)) {
                out.add(word to freq)
                if (out.size >= limit) break
            }
        }
        return out
    }

    override fun successors(token: String): Map<String, Int> =
        lcBigrams[token.lowercase()] ?: emptyMap()
}
