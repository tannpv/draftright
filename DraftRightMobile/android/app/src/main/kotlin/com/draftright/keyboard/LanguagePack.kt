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

    /**
     * Number-only keypad for numeric fields (OTP/PIN/phone, #208). Digits are
     * language-neutral, so every pack shares one definition by default; a pack
     * with a bespoke numeric arrangement may override.
     */
    val numericRows: List<List<KeyDef>> get() = com.draftright.keyboard.lang.QwertyLayout.numericRows

    /** BCP-47 tag for on-device speech recognition; null = voice input unavailable for this pack. */
    val sttLocale: String? get() = null

    /**
     * Whether pressing space converts the live composing *reading* to the top
     * candidate instead of committing the reading + a space (#207 Phase 2). True
     * for reading-conversion input (Japanese kana→kanji, Chinese pinyin→hanzi),
     * the standard JP/ZH IME behavior. False for direct/prediction input
     * (Latin, Telex, Hangul), where space is a literal space.
     */
    val convertsOnSpace: Boolean get() = false

    /**
     * Whether space auto-corrects a one-edit typo in the finished word (#207).
     * Opt-in per pack rather than per language name: it only pays off with a
     * frequency dictionary big enough to tell a typo from a rare word, so a
     * pack turns it on when it ships one. Vietnamese does; the rest don't yet.
     */
    val autoCorrectEnabled: Boolean get() = false

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
