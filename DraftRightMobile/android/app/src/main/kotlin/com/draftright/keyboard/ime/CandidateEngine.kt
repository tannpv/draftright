package com.draftright.keyboard.ime

/**
 * One ranked suggestion shown in the candidate bar. `text` is what gets
 * committed when the user taps; `display` is what the bar renders (for CJK
 * scripts the two differ — display can show okurigana hints, the committed
 * text is the kanji+kana run). For Latin-script word prediction both are
 * the same string.
 */
data class Candidate(
    val text: String,
    val display: String = text,
)

/**
 * Engine-agnostic seam between the keyboard UI and whatever produces
 * suggestions: a trigram engine for Latin scripts, a librime adapter for
 * CJK, a no-op stub for languages that ship without prediction.
 *
 * The keyboard calls [suggest] every keystroke (or every committed token,
 * for non-composing engines). Engines must return fast — a 50 ms budget on
 * a Galaxy A52 — and never block the UI thread.
 *
 * Per Rule #1: the interface is intentionally minimal so adding a new
 * engine (e.g. an on-device LLM next year) doesn't ripple through the
 * keyboard, the candidate bar, or the language pack registry.
 */
interface CandidateEngine {
    /**
     * Produce up to [limit] candidates for the current input state.
     *
     * @param composing        Live composing buffer — what the user is
     *                         actively typing in the current word (Telex for
     *                         Vietnamese, romaji for Japanese, raw ASCII
     *                         for English). Empty means "no live word".
     * @param previousTokens   Recent committed words (most-recent last),
     *                         used for next-word prediction and n-gram
     *                         context. Implementations may ignore them.
     * @param limit            Hard cap on returned candidates. Most bars
     *                         render 3–7; engines can return fewer.
     */
    fun suggest(
        composing: String,
        previousTokens: List<String> = emptyList(),
        limit: Int = 7,
    ): List<Candidate>

    /**
     * Free any data the engine memory-mapped or cached. Called when the IME
     * service is shutting down or the user disables this language.
     */
    fun close() {}
}
