package com.draftright.keyboard.ime

/**
 * Chinese candidate engine (#211): everything the shared
 * [DictionaryCandidateEngine] does (exact word lookup + raw-pinyin fallback),
 * PLUS **sentence-level pinyin** — when the run-together pinyin isn't a known
 * word, segment it and build a hanzi candidate from each syllable's top match
 * (e.g. "woshi" → 我是). Wrapping the shared engine rather than editing it keeps
 * Japanese (which uses the same [DictionaryCandidateEngine]) untouched — Rule #1.
 */
class PinyinCandidateEngine(
    private val dictionary: Map<String, List<String>>,
) : CandidateEngine {

    private val base = DictionaryCandidateEngine(dictionary)

    override fun suggest(
        composing: String,
        previousTokens: List<String>,
        limit: Int,
    ): List<Candidate> {
        if (composing.isEmpty()) return emptyList()
        val baseCands = base.suggest(composing, previousTokens, limit)
        val segmented = segmentedCandidate(composing) ?: return baseCands

        // Insert the segmented sentence just BEFORE the raw-pinyin fallback (the
        // entry whose text == the composing string), so real hanzi rank above raw
        // pinyin but below an exact dictionary word. Dedup by text.
        val out = ArrayList<Candidate>(baseCands.size + 1)
        val seen = HashSet<String>()
        for (c in baseCands) {
            if (c.text == composing && seen.add(segmented)) out.add(Candidate(segmented))
            if (seen.add(c.text)) out.add(c)
        }
        if (seen.add(segmented)) out.add(Candidate(segmented))
        return out.take(limit)
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
