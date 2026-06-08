package com.draftright.keyboard.lang

import com.draftright.keyboard.Composer
import com.draftright.keyboard.KeyDef
import com.draftright.keyboard.LanguagePack
import com.draftright.keyboard.SpecialKeys
import com.draftright.keyboard.composer.HangulComposer
import java.util.Locale

private fun jamo(vararg labels: String): List<KeyDef> =
    labels.map { KeyDef(it, it[0].code) }

/**
 * Korean (한국어) pack — dubeolsik (2-set) jamo layout. Keys carry the jamo;
 * [HangulComposer] assembles them into syllable blocks live. No candidate bar
 * (Korean is deterministic composition, not dictionary conversion).
 *
 * Long-press a basic consonant/vowel for its tense/extra form
 * (ㅂ→ㅃ, ㄱ→ㄲ, ㅐ→ㅒ, …) — the dubeolsik "shift" set.
 */
object KoreanLanguagePack : LanguagePack {
    override val id = "ko"
    override val displayName = "한국어"
    override val locale: Locale = Locale.KOREAN

    override val alphaRows: List<List<KeyDef>> = listOf(
        jamo("ㅂ", "ㅈ", "ㄷ", "ㄱ", "ㅅ", "ㅛ", "ㅕ", "ㅑ", "ㅐ", "ㅔ"),
        jamo("ㅁ", "ㄴ", "ㅇ", "ㄹ", "ㅎ", "ㅗ", "ㅓ", "ㅏ", "ㅣ"),
        listOf(KeyDef("⬆", SpecialKeys.SHIFT, 1.5f)) +
            jamo("ㅋ", "ㅌ", "ㅊ", "ㅍ", "ㅠ", "ㅜ", "ㅡ") +
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

    // Symbols layers shared with the Latin/QWERTY layout.
    override val symbols1Rows: List<List<KeyDef>> = QwertyLayout.symbols1Rows
    override val symbols2Rows: List<List<KeyDef>> = QwertyLayout.symbols2Rows

    override val longPressAccents: Map<Char, List<Char>> = mapOf(
        'ㅂ' to listOf('ㅃ'), 'ㅈ' to listOf('ㅉ'), 'ㄷ' to listOf('ㄸ'),
        'ㄱ' to listOf('ㄲ'), 'ㅅ' to listOf('ㅆ'),
        'ㅐ' to listOf('ㅒ'), 'ㅔ' to listOf('ㅖ'),
    )

    override fun composer(): Composer = HangulComposer()
}
