package com.draftright.keyboard

import com.draftright.keyboard.composer.PassthroughComposer
import com.draftright.keyboard.ime.CandidateEngine
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

    /** Default: no composition (Latin packs type directly). JP/VI override. */
    fun composer(): Composer = PassthroughComposer()

    /**
     * Suggestion engine shown in the candidate bar — Telex-aware trigram for
     * Vietnamese, prefix-trigram for Latin scripts, RIME adapter for JP/ZH/KO,
     * null to render no bar at all (the default).
     *
     * Returning the engine lazily means downloadable packs (RIME schemas, big
     * word lists) can be installed AFTER the keyboard's first paint without
     * a registry rebuild — the next syllable gets the new candidates.
     */
    fun candidateEngine(): CandidateEngine? = null
}
