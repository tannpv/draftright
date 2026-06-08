package com.draftright.keyboard.lang

import com.draftright.keyboard.Composer
import com.draftright.keyboard.KeyDef
import com.draftright.keyboard.LanguagePack
import com.draftright.keyboard.composer.RomajiKanaComposer
import com.draftright.keyboard.ime.CandidateEngine
import com.draftright.keyboard.ime.ImeContext
import com.draftright.keyboard.ime.DictionaryCandidateEngine
import com.draftright.keyboard.ime.DictPackResolver
import com.draftright.keyboard.ime.JapaneseSeedDictionary
import java.util.Locale

/**
 * Japanese (rōmaji → kana → kanji) pack — mirrors the Swift JapaneseLanguagePack.
 * Rōmaji is typed on the standard ASCII QWERTY ([QwertyLayout], shared with
 * English); intelligence comes from candidateEngine() → JapaneseDictionaryEngine.
 * Ships a built-in seed dict so it works on enable; the full reading→kanji
 * dictionary arrives later as a downloadable pack.
 */
object JapaneseLanguagePack : LanguagePack {
    override val id = "ja"
    override val displayName = "日本語"
    override val locale: Locale = Locale.JAPANESE

    override val alphaRows: List<List<KeyDef>> = QwertyLayout.alphaRows
    override val symbols1Rows: List<List<KeyDef>> = QwertyLayout.symbols1Rows
    override val symbols2Rows: List<List<KeyDef>> = QwertyLayout.symbols2Rows
    override val longPressAccents: Map<Char, List<Char>> = emptyMap()

    /** Rōmaji→kana composer; its kana buffer drives the candidate engine. */
    override fun composer(): Composer = RomajiKanaComposer()

    /** Matches the backend manifest's pack URL prefix. */
    private const val PACK_ID_PREFIX = "draftright-ime-ja"

    /**
     * Cached engine — candidateEngine() is called on EVERY keystroke, and the
     * dictionary is 700k+ entries (~27 MB). Parse the pack once per IME session,
     * not per keystroke (that was the Japanese lag). Same lazy double-check
     * pattern as EnglishLanguagePack.
     */
    @Volatile
    private var cachedEngine: CandidateEngine? = null

    override fun candidateEngine(): CandidateEngine {
        cachedEngine?.let { return it }
        synchronized(this) {
            cachedEngine?.let { return it }
            val ctx = ImeContext.appOrNull()
            val dict = if (ctx != null) {
                DictPackResolver.loadOrFallback(ctx, PACK_ID_PREFIX) { JapaneseSeedDictionary.dict }
            } else {
                JapaneseSeedDictionary.dict
            }
            val engine = DictionaryCandidateEngine(dict)
            cachedEngine = engine
            return engine
        }
    }
}
