package com.draftright.keyboard.ime

/**
 * Chinese candidate engine (#211): everything the shared
 * [DictionaryCandidateEngine] does (exact word lookup + raw-pinyin fallback),
 * PLUS **sentence-level pinyin** — when the run-together pinyin isn't a known
 * word, segment it and build a hanzi candidate from each syllable's top match
 * (e.g. "woshi" → 我是) — AND **initials abbreviation** — a word's syllable
 * initials commit the whole word (e.g. "nh" → 你好, "bj" → 北京). Wrapping the
 * shared engine rather than editing it keeps Japanese (which uses the same
 * [DictionaryCandidateEngine]) untouched — Rule #1.
 */
class PinyinCandidateEngine(
    private val dictionary: Map<String, List<String>>,
) : CandidateEngine {

    private val base = DictionaryCandidateEngine(dictionary)

    /**
     * initials ("bj") → the hanzi words whose syllable initials spell it (北京),
     * derived once from the dictionary + [PinyinSegmenter] (no new source of
     * truth — Rule #1). Built lazily so constructing the engine stays cheap.
     */
    private val initialsIndex: Map<String, List<String>> by lazy { buildInitialsIndex() }

    override fun suggest(
        composing: String,
        previousTokens: List<String>,
        limit: Int,
    ): List<Candidate> {
        if (composing.isEmpty()) return emptyList()
        val baseCands = base.suggest(composing, previousTokens, limit)

        // Candidates DERIVED from the composing pinyin, best-first: a segmented
        // sentence, then any initials-abbreviation matches.
        val derived = ArrayList<String>()
        segmentedCandidate(composing)?.let { derived.add(it) }
        initialsIndex[composing]?.let { derived.addAll(it) }
        if (derived.isEmpty()) return baseCands

        // Insert the derived candidates just BEFORE the raw-pinyin fallback (the
        // entry whose text == the composing string), so real hanzi rank above raw
        // pinyin but below an exact dictionary word. Dedup by text.
        val out = ArrayList<Candidate>(baseCands.size + derived.size)
        val seen = HashSet<String>()
        for (c in baseCands) {
            if (c.text == composing) {
                for (d in derived) if (seen.add(d)) out.add(Candidate(d))
            }
            if (seen.add(c.text)) out.add(c)
        }
        for (d in derived) if (seen.add(d)) out.add(Candidate(d))
        return out.take(limit)
    }

    private fun buildInitialsIndex(): Map<String, List<String>> {
        val index = HashMap<String, MutableList<String>>()
        for ((reading, hanziList) in dictionary) {
            val segments = PinyinSegmenter.segment(reading) ?: continue
            if (segments.size < 2) continue
            val initials = buildString { for (s in segments) append(s[0]) }
            val bucket = index.getOrPut(initials) { ArrayList() }
            for (hanzi in hanziList) if (hanzi !in bucket) bucket.add(hanzi)
        }
        return index
    }

    /**
     * A hanzi string built from segmenting [pinyin] and taking each syllable's
     * top candidate, or null when it can't be fully segmented or any syllable has
     * no hanzi. Only fires for 2+ syllables (single syllables are the base
     * engine's exact-lookup job).
     */
    private fun segmentedCandidate(pinyin: String): String? {
        if (pinyin.length <= 1) return null
        val segments = PinyinSegmenter.segment(pinyin) ?: return null
        if (segments.size < 2) return null
        val sb = StringBuilder(segments.size)
        for (syllable in segments) {
            sb.append(dictionary[syllable]?.firstOrNull() ?: return null)
        }
        return sb.toString()
    }

    override fun close() = base.close()
}
