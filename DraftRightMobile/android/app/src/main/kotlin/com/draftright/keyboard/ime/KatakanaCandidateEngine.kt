package com.draftright.keyboard.ime

/**
 * Japanese candidate engine (#211/#212): everything the shared
 * [DictionaryCandidateEngine] does (kana→kanji lookup + plain-hiragana
 * fallback), PLUS the **katakana** form of the reading (かな → カナ) — the
 * standard way every JP IME lets users write loanwords and names. Wrapping the
 * shared engine rather than editing it keeps Chinese (which uses the same
 * [DictionaryCandidateEngine]) untouched — Rule #1. Mirrors Swift
 * `KatakanaCandidateEngine`.
 */
class KatakanaCandidateEngine(
    dictionary: Map<String, List<String>>,
) : CandidateEngine {

    private val base = DictionaryCandidateEngine(dictionary)

    override fun suggest(
        composing: String,
        previousTokens: List<String>,
        limit: Int,
    ): List<Candidate> {
        if (composing.isEmpty()) return emptyList()
        val baseCands = base.suggest(composing, previousTokens, limit)
        val katakana = Katakana.fromHiragana(composing)
        // Nothing to convert (no hiragana in the buffer) or already offered.
        if (katakana == composing || baseCands.any { it.text == katakana }) return baseCands

        // Slot katakana just above the plain-hiragana fallback (kanji stay on
        // top, hiragana stays last) — the ordering real JP IMEs use.
        val out = baseCands.toMutableList()
        val readingIdx = out.indexOfFirst { it.text == composing }
        val candidate = Candidate(katakana, katakana)
        if (readingIdx >= 0) out.add(readingIdx, candidate) else out.add(candidate)
        return out.take(limit)
    }
}
