package com.draftright.keyboard.ime

/**
 * Generic reading→candidates engine: looks the live composing string up in a
 * dictionary and offers the matches, with the reading itself as a fallback.
 *
 * Reading-script conversion is the composer's job (RomajiKanaComposer produces
 * kana, PinyinComposer produces pinyin), so the SAME engine serves Japanese
 * (kana→kanji) and Chinese (pinyin→hanzi) — only the dictionary differs
 * (Rule #1). Pure lookup, no native code, mmap-friendly downloadable packs.
 */
class DictionaryCandidateEngine(
    /** reading → ranked candidates (best first). */
    private val dictionary: Map<String, List<String>>,
) : CandidateEngine {

    override fun suggest(
        composing: String,
        previousTokens: List<String>,
        limit: Int,
    ): List<Candidate> {
        if (composing.isEmpty()) return emptyList()
        val out = mutableListOf<Candidate>()
        dictionary[composing]?.forEach { out.add(Candidate(it, it)) }
        // The plain reading is always a valid commit (e.g. hiragana, or raw pinyin).
        if (out.none { it.text == composing }) out.add(Candidate(composing, composing))
        return out.take(limit)
    }
}
