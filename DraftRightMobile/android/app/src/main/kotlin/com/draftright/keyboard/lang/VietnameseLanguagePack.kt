package com.draftright.keyboard.lang

import com.draftright.draftright_mobile.v2.R
import com.draftright.keyboard.Composer
import com.draftright.keyboard.KeyDef
import com.draftright.keyboard.LanguagePack
import com.draftright.keyboard.composer.TelexComposer
import com.draftright.keyboard.ime.CandidateEngine
import com.draftright.keyboard.ime.ImeContext
import com.draftright.keyboard.ime.TrigramCandidateEngine
import com.draftright.keyboard.ime.WordListPackResolver
import java.util.Locale

object VietnameseLanguagePack : LanguagePack {
    override val id = "vi"
    override val displayName = "Tiếng Việt"
    override val locale: Locale = Locale("vi")

    override val alphaRows: List<List<KeyDef>> = EnglishLanguagePack.alphaRows
    override val symbols1Rows: List<List<KeyDef>> = EnglishLanguagePack.symbols1Rows
    override val symbols2Rows: List<List<KeyDef>> = EnglishLanguagePack.symbols2Rows
    override val longPressAccents: Map<Char, List<Char>> = emptyMap()

    override val sttLocale: String? = "vi-VN"

    override fun composer(): Composer = TelexComposer()

    /**
     * Loaded on first request, kept for the IME service's lifetime so the
     * bootstrap list doesn't re-parse every keystroke. Volatile so the
     * double-check init is safe across the IME's lifecycle threads.
     */
    @Volatile
    private var cachedEngine: CandidateEngine? = null

    /**
     * Pack id prefix matching the backend manifest's wordlistPack URL.
     * [WordListPackResolver] scans for files matching this + a `-v<N>.pack`
     * suffix and picks the latest installed version, falling back to the
     * bundled raw resource when nothing is installed yet.
     */
    private const val WORDLIST_PACK_PREFIX = "draftright-wordlist-vi"

    override fun candidateEngine(): CandidateEngine? {
        cachedEngine?.let { return it }
        val ctx = ImeContext.appOrNull() ?: return null
        synchronized(this) {
            cachedEngine?.let { return it }
            val list = WordListPackResolver.loadOrFallback(
                context = ctx,
                packIdPrefix = WORDLIST_PACK_PREFIX,
                fallbackResId = R.raw.wordlist_vi,
            )
            val engine = TrigramCandidateEngine(list)
            cachedEngine = engine
            return engine
        }
    }
}
