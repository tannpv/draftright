package com.draftright.keyboard.lang

import com.draftright.keyboard.KeyDef
import com.draftright.keyboard.LanguagePack
import com.draftright.keyboard.SpecialKeys
import java.util.Locale

private fun chars(vararg labels: String): List<KeyDef> =
    labels.map { KeyDef(it, it[0].code) }

object PortugueseLanguagePack : LanguagePack {
    override val id = "pt"
    override val displayName = "Português"
    override val locale: Locale = Locale("pt")

    override val alphaRows: List<List<KeyDef>> = listOf(
        chars("q", "w", "e", "r", "t", "y", "u", "i", "o", "p"),
        chars("a", "s", "d", "f", "g", "h", "j", "k", "l", "ç"),
        listOf(KeyDef("⬆", SpecialKeys.SHIFT, 1.5f)) +
            chars("z", "x", "c", "v", "b", "n", "m") +
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
        'a' to listOf('á', 'â', 'ã', 'à', 'ä'),
        'e' to listOf('é', 'ê', 'ë'),
        'i' to listOf('í', 'î', 'ï'),
        'o' to listOf('ó', 'ô', 'õ', 'ö'),
        'u' to listOf('ú', 'û', 'ü'),
        'c' to listOf('ç'),
    )
}
