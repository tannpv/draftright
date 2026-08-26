package com.draftright.keyboard.lang

import com.draftright.keyboard.KeyDef
import com.draftright.keyboard.SpecialKeys

/**
 * Standard ASCII QWERTY key layout shared by every Latin/rōmaji pack that
 * doesn't need a bespoke arrangement (English, Japanese rōmaji, …).
 *
 * Rule #1: the three big row tables live in ONE place instead of being copied
 * into each pack. Packs needing a different layout (e.g. French AZERTY) don't
 * use this.
 */
object QwertyLayout {
    private fun chars(vararg labels: String): List<KeyDef> =
        labels.map { KeyDef(it, it[0].code) }

    /** Shared bottom row (layer-switch · globe · , · space · . · enter). */
    private fun bottomRow(layerKey: KeyDef): List<KeyDef> = listOf(
        layerKey,
        KeyDef("🌐", SpecialKeys.GLOBE, 1.0f),
        KeyDef(",", ','.code, 1.0f),
        KeyDef(" ", ' '.code, 4.0f),
        KeyDef(".", '.'.code, 1.0f),
        KeyDef("↵", SpecialKeys.ENTER, 1.5f),
    )

    val alphaRows: List<List<KeyDef>> = listOf(
        chars("q", "w", "e", "r", "t", "y", "u", "i", "o", "p"),
        chars("a", "s", "d", "f", "g", "h", "j", "k", "l"),
        listOf(KeyDef("⬆", SpecialKeys.SHIFT, 1.5f)) +
            chars("z", "x", "c", "v", "b", "n", "m") +
            listOf(KeyDef("←", SpecialKeys.BACKSPACE, 1.5f)),
        bottomRow(KeyDef("?123", SpecialKeys.SYMBOLS, 1.5f)),
    )

    val symbols1Rows: List<List<KeyDef>> = listOf(
        chars("1", "2", "3", "4", "5", "6", "7", "8", "9", "0"),
        chars("@", "#", "$", "%", "&", "-", "_", "+", "(", ")"),
        listOf(KeyDef("#+=", SpecialKeys.SYMBOLS2, 1.5f)) +
            chars("!", "\"", "'", ":", ";", "/", "?") +
            listOf(KeyDef("←", SpecialKeys.BACKSPACE, 1.5f)),
        bottomRow(KeyDef("ABC", SpecialKeys.ALPHA, 1.5f)),
    )

    /**
     * Samsung-style number-only keypad for OTP/PIN/phone fields (#208). A 3-col
     * digit grid — NOT the ?123 symbols layer, so no letters or punctuation
     * distract from entering a code. Language-neutral, so every pack shares this
     * one definition (Rule #1). Bottom row keeps ABC (escape back to letters),
     * 0, backspace and enter so the user is never trapped on the pad.
     */
    val numericRows: List<List<KeyDef>> = listOf(
        chars("1", "2", "3"),
        chars("4", "5", "6"),
        chars("7", "8", "9"),
        listOf(
            KeyDef("ABC", SpecialKeys.ALPHA, 1.0f),
            KeyDef("0", '0'.code, 1.0f),
            KeyDef("←", SpecialKeys.BACKSPACE, 1.0f),
            KeyDef("↵", SpecialKeys.ENTER, 1.0f),
        ),
    )

    val symbols2Rows: List<List<KeyDef>> = listOf(
        chars("~", "`", "|", "•", "√", "π", "÷", "×", "¶", "Δ"),
        chars("£", "€", "¥", "^", "[", "]", "{", "}"),
        listOf(KeyDef("?123", SpecialKeys.SYMBOLS, 1.5f)) +
            chars("©", "®", "™", "\\", "<", ">", "=") +
            listOf(KeyDef("←", SpecialKeys.BACKSPACE, 1.5f)),
        bottomRow(KeyDef("ABC", SpecialKeys.ALPHA, 1.5f)),
    )
}
