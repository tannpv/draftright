package com.draftright.keyboard.composer

import com.draftright.keyboard.ComposeResult

/**
 * Feeds a key string to a fresh [TelexComposer] one character at a time and
 * returns what the user would see. Shared by every Telex suite that works in
 * keystrokes (order-free, vectors, exhaustive) so the driver itself is one
 * source of truth. Mirror of Swift `TelexTyping`.
 */
object TelexTyping {
    fun type(keys: String): String {
        val composer = TelexComposer()
        var out = ""
        for (k in keys) {
            when (val r = composer.onKey(k)) {
                is ComposeResult.Composing -> out = r.text
                is ComposeResult.Commit -> out = r.text
                else -> {}
            }
        }
        return out
    }
}
