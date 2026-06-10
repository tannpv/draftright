package com.draftright.keyboard.ime

/**
 * Suggestion engine for Latin-script languages — Vietnamese, English,
 * French, Spanish, German, Italian, Portuguese. Backed by a static word
 * frequency table plus an optional next-word trigram model; intended to
 * mmap a pack downloaded via the same ImePackService that delivers
 * the RIME schemas for CJK languages.
 *
 * Algorithm (cheap, runs every keystroke):
 *   1. Word completion — prefix-match the composing buffer against the
 *      word list, ordered by frequency.
 *   2. Next-word prediction — when [composing] is empty and we have at
 *      least one previousToken, look up `previousTokens.last()` in the
 *      bigram table and return the top successors.
 *   3. Merge + truncate to [limit].
 *
 * The dictionary surface is a [LanguageWordList]: an alphabetized array of
 * (word, freq) pairs and a sparse bigram map. Both come from the language
 * pack — see the IME-framework plan, Task 11.
 *
 * Why a trigram and not a neural model: a static n-gram is two orders of
 * magnitude smaller (a few MB vs 100+) and runs in microseconds on every
 * keystroke without battery cost. The seam (CandidateEngine) means we can
 * swap to a small LM later without UI changes.
 */
class TrigramCandidateEngine(private val wordList: LanguageWordList) : CandidateEngine {

    override fun suggest(
        composing: String,
        previousTokens: List<String>,
        limit: Int,
    ): List<Candidate> {
        if (limit <= 0) return emptyList()
        return if (composing.isEmpty()) {
            nextWord(previousTokens, limit)
        } else {
            completions(composing, previousTokens, limit)
        }
    }

    private fun completions(
        prefix: String,
        previousTokens: List<String>,
        limit: Int,
    ): List<Candidate> {
        val matches = wordList.prefixMatches(prefix, limit * 2)
        // Boost candidates that are valid successors of the last token.
        val lastToken = previousTokens.lastOrNull()
        val successors = lastToken?.let { wordList.successors(it) } ?: emptyMap()
        val ranked = matches.sortedByDescending { (word, freq) ->
            val bigramBoost = successors[word] ?: 0
            freq + bigramBoost * BIGRAM_WEIGHT
        }
        return ranked.take(limit).map { (word, _) -> Candidate(word) }
    }

    private fun nextWord(previousTokens: List<String>, limit: Int): List<Candidate> {
        val last = previousTokens.lastOrNull() ?: return emptyList()
        val successors = wordList.successors(last)
        if (successors.isEmpty()) return emptyList()
        return successors.entries
            .sortedByDescending { it.value }
            .take(limit)
            .map { Candidate(it.key) }
    }

    override fun close() {
        wordList.close()
    }

    companion object {
        /** Bigram score weight relative to unigram frequency. Tuned empirically. */
        const val BIGRAM_WEIGHT = 5
    }
}
