package com.draftright.keyboard.lang

import com.draftright.keyboard.Composer
import com.draftright.keyboard.KeyDef
import com.draftright.keyboard.LanguagePack
import com.draftright.keyboard.composer.TelexComposer
import java.util.Locale

object VietnameseLanguagePack : LanguagePack {
    override val id = "vi"
    override val displayName = "Tiếng Việt"
    override val locale: Locale = Locale("vi")

    override val alphaRows: List<List<KeyDef>> = EnglishLanguagePack.alphaRows
    override val symbols1Rows: List<List<KeyDef>> = EnglishLanguagePack.symbols1Rows
    override val symbols2Rows: List<List<KeyDef>> = EnglishLanguagePack.symbols2Rows
    override val longPressAccents: Map<Char, List<Char>> = emptyMap()

    override fun composer(): Composer = TelexComposer()
}
