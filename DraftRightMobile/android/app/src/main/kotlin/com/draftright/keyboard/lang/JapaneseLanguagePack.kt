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
     * Cached engine + the pack identity it was built from. candidateEngine() is
     * called on EVERY keystroke and the dictionary is 700k+ entries (~27 MB), so
     * keep the parsed engine across keystrokes — but key the cache on the
     * resolved pack file (its name encodes the version) so a pack installed
     * mid-session rebuilds instead of serving the stale dict (issue #10). The
     * 27 MB parse still happens only on an actual pack change, not per key.
     */
    @Volatile
    private var cachedEngine: CandidateEngine? = null
    @Volatile
    private var cachedKey: String? = null

    override fun candidateEngine(): CandidateEngine {
        val ctx = ImeContext.appOrNull()
        val key = if (ctx != null) {
            DictPackResolver.resolvedPackFile(ctx, PACK_ID_PREFIX)?.path ?: "seed"
        } else {
            "seed"
        }
        cachedEngine?.let { if (key == cachedKey) return it }
        synchronized(this) {
            cachedEngine?.let { if (key == cachedKey) return it }
            val dict = if (ctx != null) {
                DictPackResolver.loadOrFallback(ctx, PACK_ID_PREFIX) { JapaneseSeedDictionary.dict }
            } else {
                JapaneseSeedDictionary.dict
            }
            val engine = DictionaryCandidateEngine(dict)
            cachedEngine = engine
            cachedKey = key
            return engine
        }
    }
}
