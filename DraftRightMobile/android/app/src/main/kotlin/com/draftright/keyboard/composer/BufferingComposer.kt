package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult
import com.draftright.keyboard.Composer

/**
 * Shared skeleton for composers that accumulate raw input into a buffer and
 * expose a transformed "display" (composing) string — e.g. rōmaji→kana for
 * Japanese, pinyin for Chinese. Subclasses implement only [transform]; the
 * keystroke/backspace/commit/reset machinery lives here once (Rule #1).
 *
 * Not used by TelexComposer, whose Vietnamese combining happens per-keystroke
 * on the live buffer rather than as a whole-buffer transform.
 */
abstract class BufferingComposer : Composer {

    private val raw = StringBuilder()
    private var cachedRaw: String? = null
    private var cachedDisplay: String = ""

    /** Convert the accumulated raw input to the composing/display text. */
    protected abstract fun transform(raw: String): String

    /** Which characters are buffered as input. Default: letters. */
    protected open fun isInputChar(char: Char): Boolean = char.isLetter()

    /** Max raw length before auto-committing, to bound the buffer. */
    protected open val maxLen: Int = 48

    /** Memoized transform — recomputed only when the raw buffer changes, so
     *  currentComposingText()/onKey don't re-run the conversion every call. */
    private fun display(): String {
        val r = raw.toString()
        if (cachedRaw != r) {
            cachedRaw = r
            cachedDisplay = transform(r)
        }
        return cachedDisplay
    }

    override fun onKey(char: Char): ComposeResult {
        if (raw.length >= maxLen) {
            val committed = display()
            raw.clear()
            if (isInputChar(char)) {
                raw.append(char)
                return ComposeResult.Composing(display())
            }
            return ComposeResult.Commit(committed + char)
        }
        if (!isInputChar(char)) {
            if (raw.isEmpty()) return ComposeResult.PassThrough
            val out = display() + char
            raw.clear()
            return ComposeResult.Commit(out)
        }
        raw.append(char)
        return ComposeResult.Composing(display())
    }

    override fun onBackspace(): ComposeResult {
        if (raw.isEmpty()) return ComposeResult.PassThrough
        raw.deleteCharAt(raw.length - 1)
        return if (raw.isEmpty()) ComposeResult.Consumed else ComposeResult.Composing(display())
    }

    override fun reset() {
        raw.clear()
        cachedRaw = null
        cachedDisplay = ""
    }

    override fun currentComposingText(): String = display()
}
