package com.draftright.keyboard.ime

/**
 * Pure typo→correction decision for auto-correct-on-space (#207): given the
 * token the user just finished, return the word to commit instead, or `null`
 * to leave it exactly as typed.
 *
 * Deliberately conservative — a wrong correction is far more annoying than a
 * missed one, so it only fires when the token is not itself a word, has a
 * single candidate within [MAX_EDITS], and that candidate is both common and
 * clearly ahead of its runner-up.
 *
 * Reuses [LanguageWordList.fuzzyMatches] for edit distance; there must never be
 * a second Levenshtein in this codebase. That also sets the limit of what gets
 * corrected: plain Levenshtein counts a transposition ("khôgn") as two edits,
 * so swapped letters are out of reach until the shared distance function grows
 * a transposition case on both platforms. Mirror of Swift `AutoCorrector` —
 * the thresholds below are asserted equal by
 * `scripts/check-autocorrect-consts-parity.py`, and both sides run the shared
 * `parity/autocorrect-vectors.json` cases.
 */
object AutoCorrector {
    /** Only single-typo slips are corrected; 2+ edits are guesswork. */
    const val MAX_EDITS = 1

    /** A candidate rarer than this isn't worth overriding the user for. */
    const val MIN_CONFIDENCE_FREQ = 500

    /** The winner must be this many times more frequent than the runner-up. */
    const val MIN_CONFIDENCE_MARGIN = 4

    /**
     * The corrected word for [token], or `null` to leave it as typed.
     * Casing of [token] is preserved: a leading capital carries over to the
     * correction, and anything less regular (ALL CAPS, mIxEd) is left alone
     * because it's more likely an acronym or a name than a typo.
     */
    fun correct(token: String, words: LanguageWordList): String? {
        if (token.isEmpty() || token.any { !it.isLetter() }) return null
        val lower = token.lowercase()
        val capitalized = token == lower.replaceFirstChar { it.uppercaseChar() }
        if (token != lower && !capitalized) return null

        if (words.frequencyOf(lower) > 0) return null // already a real word
        val candidates = words.fuzzyMatches(lower, MAX_EDITS, limit = 2)
        val (word, freq) = candidates.firstOrNull() ?: return null
        if (freq < MIN_CONFIDENCE_FREQ) return null
        val runnerUp = candidates.getOrNull(1)?.second ?: 0
        if (freq < runnerUp * MIN_CONFIDENCE_MARGIN) return null // too close to call

        return if (capitalized) word.replaceFirstChar { it.uppercaseChar() } else word
    }
}
