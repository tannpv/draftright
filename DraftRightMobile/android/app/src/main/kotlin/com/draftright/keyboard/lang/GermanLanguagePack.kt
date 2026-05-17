package com.draftright.keyboard.lang

import com.draftright.keyboard.KeyDef
import com.draftright.keyboard.LanguagePack
import com.draftright.keyboard.SpecialKeys
import java.util.Locale

private fun chars(vararg labels: String): List<KeyDef> =
    labels.map { KeyDef(it, it[0].code) }

object GermanLanguagePack : LanguagePack {
    override val id = "de"
    override val displayName = "Deutsch"
    override val locale: Locale = Locale.GERMAN

    override val alphaRows: List<List<KeyDef>> = listOf(
        chars("q", "w", "e", "r", "t", "z", "u", "i", "o", "p", "ü"),
        chars("a", "s", "d", "f", "g", "h", "j", "k", "l", "ö", "ä"),
        listOf(KeyDef("⬆", SpecialKeys.SHIFT, 1.5f)) +
            chars("y", "x", "c", "v", "b", "n", "m", "ß") +
            listOf(KeyDef("←", SpecialKeys.BACKSPACE, 1.5f)),
        listOf(
            KeyDef("?123", SpecialKeys.SYMBOLS, 1.5f),
            KeyDef("🌐", SpecialKeys.GLOBE, 1.0f),
            KeyDef("≡", SpecialKeys.GLOBE_PICKER, 1.0f),
            KeyDef(",", ','.code, 1.0f),
            KeyDef(" ", ' '.code, 4.0f),
            KeyDef(".", '.'.code, 1.0f),
            KeyDef("↵", SpecialKeys.ENTER, 1.5f),
        ),
    )

    override val symbols1Rows: List<List<KeyDef>> = EnglishLanguagePack.symbols1Rows
    override val symbols2Rows: List<List<KeyDef>> = EnglishLanguagePack.symbols2Rows

    override val longPressAccents: Map<Char, List<Char>> = mapOf(
        'a' to listOf('à', 'á', 'â', 'ã'),
        'e' to listOf('é', 'è', 'ê', 'ë'),
        'i' to listOf('í', 'ì', 'î', 'ï'),
        'o' to listOf('ó', 'ò', 'ô', 'õ'),
        'u' to listOf('ú', 'ù', 'û'),
    )
}
