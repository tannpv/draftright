package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult
import com.draftright.keyboard.Composer
import com.draftright.keyboard.ime.RomajiComposer

/**
 * Composer for Japanese: accumulates typed rōmaji and exposes the live kana
 * conversion as the composing buffer. The candidate engine
 * (JapaneseDictionaryEngine) reads `currentComposingText()` (= kana) and offers
 * kanji candidates; tapping one commits via the IME's candidate handler.
 *
 * Mirrors the TelexComposer contract so the keyboard's keystroke pipeline is
 * unchanged (Rule #1) — the only Japanese-specific bit is rōmaji→kana, which is
 * delegated to the shared [RomajiComposer].
 */
class RomajiKanaComposer : Composer {

    private val raw = StringBuilder()

    private fun kana(): String {
        val c = RomajiComposer()
        c.feed(raw.toString())
        return c.text()
    }

    override fun onKey(char: Char): ComposeResult {
        if (raw.length >= MAX_LEN) {
            val committed = kana()
            raw.clear()
            if (char.isLetter()) {
                raw.append(char)
                return ComposeResult.Composing(kana())
            }
            return ComposeResult.Commit(committed + char)
        }

        if (!char.isLetter()) {
            // Non-letter (space/punct/digit): commit the kana so far + the char.
            return if (raw.isEmpty()) {
                ComposeResult.PassThrough
            } else {
                val out = kana() + char
                raw.clear()
                ComposeResult.Commit(out)
            }
        }

        raw.append(char)
        return ComposeResult.Composing(kana())
    }

    override fun onBackspace(): ComposeResult {
        if (raw.isEmpty()) return ComposeResult.PassThrough
        raw.deleteCharAt(raw.length - 1)
        return if (raw.isEmpty()) ComposeResult.Consumed
        else ComposeResult.Composing(kana())
    }

    override fun reset() {
        raw.clear()
    }

    override fun currentComposingText(): String = kana()

    companion object {
        const val MAX_LEN = 48
    }
}
