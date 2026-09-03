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

    /**
     * Dictionary words within [maxEdits] Levenshtein distance of [term], for
     * typo tolerance / autocorrect (#207). Used only to top up the candidate
     * bar when the exact prefix scan leaves empty slots, so a mistyped word
     * still offers the right correction. Default empty — an mmap-backed store
     * would need its own index, so it opts in rather than scanning 50k rows.
     */
    fun fuzzyMatches(term: String, maxEdits: Int, limit: Int): List<Pair<String, Int>> = emptyList()

    /**
     * Frequency of [word] as an exact dictionary entry, `0` when absent — i.e.
     * "is this a real word, and how common is it", the question auto-correct
     * (#207) asks before deciding a token is a typo. Case-insensitive, like
     * [prefixMatches] and [fuzzyMatches]. Default `0` so a store without an
     * exact index (mmap) stays valid until it overrides.
     */
    fun frequencyOf(word: String): Int = 0

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

    /**
     * Exact-lookup index over the same words, keyed lowercase. Duplicate
     * spellings keep the highest frequency so a low-frequency dupe can't hide
     * a common word from auto-correct.
     */
    private val freqByWord: Map<String, Int> =
        words.groupingBy { (word, _) -> word.lowercase() }
            .fold(0) { acc, (_, freq) -> maxOf(acc, freq) }

    override fun successors(token: String): Map<String, Int> =
        lcBigrams[token.lowercase()] ?: emptyMap()

    override fun frequencyOf(word: String): Int = freqByWord[word.lowercase()] ?: 0

    /**
     * Full scan for words within [maxEdits] of [term], ranked by (distance asc,
     * frequency desc). Cheap for the hundreds-of-entries bootstrap list; the
     * mmap store overrides with an index instead of scanning. Compares on the
     * lowercased forms so casing isn't counted as an edit.
     */
    override fun fuzzyMatches(term: String, maxEdits: Int, limit: Int): List<Pair<String, Int>> {
        if (term.isEmpty() || limit <= 0 || maxEdits <= 0) return emptyList()
        val lc = term.lowercase()
        val hits = ArrayList<Triple<String, Int, Int>>() // word, freq, distance
        for ((word, freq) in words) {
            val d = boundedLevenshtein(lc, word.lowercase(), maxEdits)
            if (d in 1..maxEdits) hits.add(Triple(word, freq, d))
        }
        hits.sortWith(compareBy({ it.third }, { -it.second }))
        return hits.take(limit).map { it.first to it.second }
    }

    /** Levenshtein distance, short-circuiting to [max]+1 once every cell in a
     *  row exceeds [max] (so a far word costs O(max·len), not O(len²)). */
    private fun boundedLevenshtein(a: String, b: String, max: Int): Int {
        if (kotlin.math.abs(a.length - b.length) > max) return max + 1
        var prev = IntArray(b.length + 1) { it }
        var curr = IntArray(b.length + 1)
        for (i in 1..a.length) {
            curr[0] = i
            var rowMin = curr[0]
            for (j in 1..b.length) {
                val cost = if (a[i - 1] == b[j - 1]) 0 else 1
                curr[j] = minOf(prev[j] + 1, curr[j - 1] + 1, prev[j - 1] + cost)
                if (curr[j] < rowMin) rowMin = curr[j]
            }
            if (rowMin > max) return max + 1
            val tmp = prev; prev = curr; curr = tmp
        }
        return prev[b.length]
    }
}
