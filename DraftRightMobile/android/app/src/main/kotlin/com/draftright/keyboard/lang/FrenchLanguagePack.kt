package com.draftright.keyboard.lang

import com.draftright.draftright_mobile.v2.R
import com.draftright.keyboard.KeyDef
import com.draftright.keyboard.LanguagePack
import com.draftright.keyboard.SpecialKeys
import com.draftright.keyboard.ime.CandidateEngine
import com.draftright.keyboard.ime.ImeContext
import com.draftright.keyboard.ime.TrigramCandidateEngine
import com.draftright.keyboard.ime.WordListPackResolver
import java.util.Locale

private fun chars(vararg labels: String): List<KeyDef> =
    labels.map { KeyDef(it, it[0].code) }

object FrenchLanguagePack : LanguagePack {
    override val id = "fr"
    override val displayName = "Français"
    override val locale: Locale = Locale.FRENCH

    override val alphaRows: List<List<KeyDef>> = listOf(
        chars("a", "z", "e", "r", "t", "y", "u", "i", "o", "p"),
        chars("q", "s", "d", "f", "g", "h", "j", "k", "l", "m"),
        listOf(KeyDef("⬆", SpecialKeys.SHIFT, 1.5f)) +
            chars("w", "x", "c", "v", "b", "n") +
            listOf(KeyDef("←", SpecialKeys.BACKSPACE, 1.5f)),
        listOf(
            KeyDef("?123", SpecialKeys.SYMBOLS, 1.5f),
            KeyDef("🌐", SpecialKeys.GLOBE, 1.0f),
            KeyDef(",", ','.code, 1.0f),
            KeyDef(" ", ' '.code, 4.0f),
            KeyDef(".", '.'.code, 1.0f),
            KeyDef("↵", SpecialKeys.ENTER, 1.5f),
        ),
    )

    override val symbols1Rows: List<List<KeyDef>> = EnglishLanguagePack.symbols1Rows
    override val symbols2Rows: List<List<KeyDef>> = EnglishLanguagePack.symbols2Rows

    override val longPressAccents: Map<Char, List<Char>> = mapOf(
        'a' to listOf('à', 'â', 'ä', 'á', 'ã'),
        'e' to listOf('é', 'è', 'ê', 'ë'),
        'i' to listOf('î', 'ï', 'í', 'ì'),
        'o' to listOf('ô', 'ö', 'ó', 'ò', 'õ'),
        'u' to listOf('ù', 'û', 'ü', 'ú'),
        'c' to listOf('ç'),
        'y' to listOf('ÿ'),
    )

    @Volatile
    private var cachedEngine: CandidateEngine? = null
    private const val WORDLIST_PACK_PREFIX = "draftright-wordlist-fr"

    override fun candidateEngine(): CandidateEngine? {
        cachedEngine?.let { return it }
        val ctx = ImeContext.appOrNull() ?: return null
        synchronized(this) {
            cachedEngine?.let { return it }
            val list = WordListPackResolver.loadOrFallback(
                context = ctx,
                packIdPrefix = WORDLIST_PACK_PREFIX,
                fallbackResId = R.raw.wordlist_fr,
            )
            val engine = TrigramCandidateEngine(list)
            cachedEngine = engine
            return engine
        }
    }
}
