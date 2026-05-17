package com.draftright.keyboard

import java.util.Locale

data class KeyDef(
    val label: String,
    val code: Int,
    val widthWeight: Float = 1.0f,
)

interface LanguagePack {
    val id: String
    val displayName: String
    val locale: Locale
    val alphaRows: List<List<KeyDef>>
    val symbols1Rows: List<List<KeyDef>>
    val symbols2Rows: List<List<KeyDef>>
    val longPressAccents: Map<Char, List<Char>>

    fun composer(): Composer? = null
}
