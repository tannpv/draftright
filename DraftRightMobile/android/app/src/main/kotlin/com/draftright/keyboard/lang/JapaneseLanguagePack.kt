package com.draftright.keyboard.lang

import com.draftright.keyboard.KeyDef
import com.draftright.keyboard.LanguagePack
import com.draftright.keyboard.ime.CandidateEngine
import com.draftright.keyboard.ime.ImeContext
import com.draftright.keyboard.ime.JapaneseDictionaryEngine
import com.draftright.keyboard.ime.JapanesePackResolver
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

    /** Matches the backend manifest's pack URL prefix. */
    private const val PACK_ID_PREFIX = "draftright-ime-ja"

    override fun candidateEngine(): CandidateEngine {
        val ctx = ImeContext.appOrNull()
        val dict = if (ctx != null) {
            JapanesePackResolver.loadOrFallback(ctx, PACK_ID_PREFIX) { JapaneseSeedDictionary.dict }
        } else {
            JapaneseSeedDictionary.dict
        }
        return JapaneseDictionaryEngine(dict)
    }
}
