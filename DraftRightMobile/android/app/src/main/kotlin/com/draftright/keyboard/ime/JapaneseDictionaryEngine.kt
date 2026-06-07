package com.draftright.keyboard.ime

/**
 * Japanese kana→kanji candidate engine — the lightweight alternative to
 * bridging librime into the Android IME (and matching the iOS Swift impl).
 *
 * Composes the existing [RomajiComposer] (rōmaji→kana) with a reading→kanji
 * dictionary (a downloadable, mmap-friendly pack in the same framework the
 * Latin word-lists already use). Pure Kotlin, no JNI/NDK, no GPL dictionary.
 *
 * Implements the engine-agnostic [CandidateEngine] seam, so the keyboard,
 * candidate bar, and pack registry are untouched.
 */
class JapaneseDictionaryEngine(
    /** kana reading → ranked kanji candidates (best first). */
    private val dictionary: Map<String, List<String>>,
) : CandidateEngine {

    override fun suggest(
        composing: String,
        previousTokens: List<String>,
        limit: Int,
    ): List<Candidate> {
        if (composing.isEmpty()) return emptyList()

        // rōmaji → kana via the shared composer (fresh instance = stateless call).
        val composer = RomajiComposer()
        composer.feed(composing)
        val kana = composer.text()
        if (kana.isEmpty()) return emptyList()

        val out = mutableListOf<Candidate>()
        // Exact reading → kanji candidates, in stored rank order.
        dictionary[kana]?.forEach { out.add(Candidate(it, it)) }
        // The plain kana is always a valid commit (user may want hiragana).
        if (out.none { it.text == kana }) out.add(Candidate(kana, kana))
        return out.take(limit)
    }
}
