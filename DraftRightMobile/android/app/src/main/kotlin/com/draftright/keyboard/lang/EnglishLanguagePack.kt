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

object EnglishLanguagePack : LanguagePack {
    override val id = "en"
    override val displayName = "English"
    override val locale: Locale = Locale.ENGLISH

    override val alphaRows: List<List<KeyDef>> = listOf(
        chars("q", "w", "e", "r", "t", "y", "u", "i", "o", "p"),
        chars("a", "s", "d", "f", "g", "h", "j", "k", "l"),
        listOf(KeyDef("⬆", SpecialKeys.SHIFT, 1.5f)) +
            chars("z", "x", "c", "v", "b", "n", "m") +
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

    override val symbols1Rows: List<List<KeyDef>> = listOf(
        chars("1", "2", "3", "4", "5", "6", "7", "8", "9", "0"),
        chars("@", "#", "$", "%", "&", "-", "+", "(", ")"),
        listOf(KeyDef("#+=", SpecialKeys.SYMBOLS2, 1.5f)) +
            chars("!", "\"", "'", ":", ";", "/", "?") +
            listOf(KeyDef("←", SpecialKeys.BACKSPACE, 1.5f)),
        listOf(
            KeyDef("ABC", SpecialKeys.ALPHA, 1.5f),
            KeyDef("🌐", SpecialKeys.GLOBE, 1.0f),
            KeyDef(",", ','.code, 1.0f),
            KeyDef(" ", ' '.code, 4.0f),
            KeyDef(".", '.'.code, 1.0f),
            KeyDef("↵", SpecialKeys.ENTER, 1.5f),
        ),
    )

    override val symbols2Rows: List<List<KeyDef>> = listOf(
        chars("~", "`", "|", "•", "√", "π", "÷", "×", "¶", "Δ"),
        chars("£", "€", "¥", "^", "[", "]", "{", "}"),
        listOf(KeyDef("?123", SpecialKeys.SYMBOLS, 1.5f)) +
            chars("©", "®", "™", "\\", "<", ">", "=") +
            listOf(KeyDef("←", SpecialKeys.BACKSPACE, 1.5f)),
        listOf(
            KeyDef("ABC", SpecialKeys.ALPHA, 1.5f),
            KeyDef("🌐", SpecialKeys.GLOBE, 1.0f),
            KeyDef(",", ','.code, 1.0f),
            KeyDef(" ", ' '.code, 4.0f),
            KeyDef(".", '.'.code, 1.0f),
            KeyDef("↵", SpecialKeys.ENTER, 1.5f),
        ),
    )

    override val longPressAccents: Map<Char, List<Char>> = emptyMap()

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
