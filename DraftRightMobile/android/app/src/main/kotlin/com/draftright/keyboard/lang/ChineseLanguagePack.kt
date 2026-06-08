package com.draftright.keyboard.lang

import com.draftright.keyboard.Composer
import com.draftright.keyboard.KeyDef
import com.draftright.keyboard.LanguagePack
import com.draftright.keyboard.composer.PinyinComposer
import com.draftright.keyboard.ime.CandidateEngine
import com.draftright.keyboard.ime.ChinesePinyinSeedDictionary
import com.draftright.keyboard.ime.DictPackResolver
import com.draftright.keyboard.ime.DictionaryCandidateEngine
import com.draftright.keyboard.ime.ImeContext
import java.util.Locale

/**
 * Chinese (中文) pinyin pack — type pinyin on the standard QWERTY ([QwertyLayout]);
 * [PinyinComposer] shows the pinyin live and [DictionaryCandidateEngine] offers
 * Hanzi candidates. Same shape as Japanese, only the dictionary differs (Rule #1).
 *
 * Ships a built-in seed; the full pinyin→hanzi dictionary arrives later as a
 * downloadable pack (mirrors the Japanese dict-pack pipeline).
 */
object ChineseLanguagePack : LanguagePack {
    override val id = "zh"
    override val displayName = "中文"
    override val locale: Locale = Locale.CHINESE

    override val alphaRows: List<List<KeyDef>> = QwertyLayout.alphaRows
    override val symbols1Rows: List<List<KeyDef>> = QwertyLayout.symbols1Rows
    override val symbols2Rows: List<List<KeyDef>> = QwertyLayout.symbols2Rows
    override val longPressAccents: Map<Char, List<Char>> = emptyMap()

    override fun composer(): Composer = PinyinComposer()

    /** Matches the backend manifest's pack URL prefix (zh pack ships later). */
    private const val PACK_ID_PREFIX = "draftright-ime-zh"

    /** Cached engine — parse the pinyin pack once per session, not per keystroke. */
    @Volatile
    private var cachedEngine: CandidateEngine? = null

    override fun candidateEngine(): CandidateEngine {
        cachedEngine?.let { return it }
        synchronized(this) {
            cachedEngine?.let { return it }
            val ctx = ImeContext.appOrNull()
            val dict = if (ctx != null) {
                DictPackResolver.loadOrFallback(ctx, PACK_ID_PREFIX) { ChinesePinyinSeedDictionary.dict }
            } else {
                ChinesePinyinSeedDictionary.dict
            }
            val engine = DictionaryCandidateEngine(dict)
            cachedEngine = engine
            return engine
        }
    }
}
