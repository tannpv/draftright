package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult
import com.draftright.keyboard.Composer

/**
 * No-op composer for languages that type directly with no composition
 * (English and the other Latin packs). Every key/backspace passes through to
 * the input field unchanged; there is no composing buffer.
 *
 * Lets every LanguagePack return a real Composer (uniform pipeline) instead of
 * null — the keyboard never special-cases "no composer". Behaviour is identical
 * to the previous null path (KeyboardController maps PassThrough → Commit char /
 * DeleteOne).
 */
class PassthroughComposer : Composer {
    override fun onKey(char: Char): ComposeResult = ComposeResult.PassThrough
    override fun onBackspace(): ComposeResult = ComposeResult.PassThrough
    override fun reset() {}
    override fun currentComposingText(): String = ""
}
