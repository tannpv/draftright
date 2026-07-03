package com.draftright.keyboard.lang

import com.draftright.draftright_mobile.v2.R
import com.draftright.keyboard.KeyDef
import com.draftright.keyboard.LanguagePack
import com.draftright.keyboard.ime.CandidateEngine
import com.draftright.keyboard.ime.ImeContext
import com.draftright.keyboard.ime.TrigramCandidateEngine
import com.draftright.keyboard.ime.WordListPackResolver
import java.util.Locale

object EnglishLanguagePack : LanguagePack {
    override val id = "en"
    override val displayName = "English"
    override val locale: Locale = Locale.ENGLISH

    // Standard ASCII QWERTY, shared with the Japanese rōmaji pack (Rule #1).
    override val alphaRows: List<List<KeyDef>> = QwertyLayout.alphaRows
    override val symbols1Rows: List<List<KeyDef>> = QwertyLayout.symbols1Rows
    override val symbols2Rows: List<List<KeyDef>> = QwertyLayout.symbols2Rows

    override val longPressAccents: Map<Char, List<Char>> = emptyMap()

    override val sttLocale: String? = "en-US"

    /**
     * Cached engine — same lazy double-check pattern as VietnameseLanguagePack.
     * One InMemoryWordList per English IME session, regardless of how many
     * times the pack's candidateEngine() is queried.
     */
    @Volatile
    private var cachedEngine: CandidateEngine? = null

    /** Matches the backend manifest's wordlistPack URL prefix. */
    private const val WORDLIST_PACK_PREFIX = "draftright-wordlist-en"

    override fun candidateEngine(): CandidateEngine? {
        cachedEngine?.let { return it }
        val ctx = ImeContext.appOrNull() ?: return null
        synchronized(this) {
            cachedEngine?.let { return it }
            val list = WordListPackResolver.loadOrFallback(
                context = ctx,
                packIdPrefix = WORDLIST_PACK_PREFIX,
                fallbackResId = R.raw.wordlist_en,
            )
            val engine = TrigramCandidateEngine(list)
            cachedEngine = engine
            return engine
        }
    }
}
