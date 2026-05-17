package com.draftright.keyboard.lang

import com.draftright.keyboard.KeyDef
import com.draftright.keyboard.LanguagePack
import java.util.Locale

object ItalianLanguagePack : LanguagePack {
    override val id = "it"
    override val displayName = "Italiano"
    override val locale: Locale = Locale.ITALIAN

    override val alphaRows: List<List<KeyDef>> = EnglishLanguagePack.alphaRows
    override val symbols1Rows: List<List<KeyDef>> = EnglishLanguagePack.symbols1Rows
    override val symbols2Rows: List<List<KeyDef>> = EnglishLanguagePack.symbols2Rows

    override val longPressAccents: Map<Char, List<Char>> = mapOf(
        'a' to listOf('à', 'á', 'â', 'ã', 'ä'),
        'e' to listOf('è', 'é', 'ê', 'ë'),
        'i' to listOf('ì', 'í', 'î', 'ï'),
        'o' to listOf('ò', 'ó', 'ô', 'õ', 'ö'),
        'u' to listOf('ù', 'ú', 'û', 'ü'),
    )
}
