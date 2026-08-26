package com.draftright.keyboard

/**
 * Extracts the recent committed words before the cursor to feed
 * [com.draftright.keyboard.ime.CandidateEngine.suggest]'s `previousTokens`
 * (next-word prediction + bigram boosting). Pure string logic so it is
 * unit-tested without an InputConnection, and shared by every language pack —
 * not Vietnamese-specific (Rule #1).
 *
 * The live composing word is excluded: it is the word being typed now, not a
 * *previous* token, and including it would make the engine predict the current
 * word as its own successor.
 */
object PreviousTokens {

    /** Trigram context depth — the engine only uses the most-recent token today,
     *  but keeping a few gives headroom for a longer context model behind the
     *  same seam without touching callers. */
    const val MAX = 3

    fun fromTextBeforeCursor(
        textBeforeCursor: CharSequence?,
        composing: String,
        max: Int = MAX,
    ): List<String> {
        var s = textBeforeCursor?.toString() ?: return emptyList()
        // Strip the live composing suffix — but only when it really is the tail,
        // so a composer that diverged from the field can't chop committed text.
        if (composing.isNotEmpty() && s.endsWith(composing)) {
            s = s.substring(0, s.length - composing.length)
        }
        return s.split(WHITESPACE)
            .map { token -> token.trim { !it.isLetter() } } // drop attached punctuation
            .filter { it.isNotEmpty() }
            .takeLast(max)
    }

    private val WHITESPACE = Regex("\\s+")
}
