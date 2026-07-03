package com.draftright.keyboard

import androidx.annotation.DrawableRes
import com.draftright.draftright_mobile.v2.R

/**
 * Single place that maps a logical icon name to its Material vector drawable.
 *
 * Names come from two sources — [ShiftState.iconName] for the shift key and
 * [Tone.iconRes] for the toolbar — so adding a new tone or key icon means one
 * drawable file plus one line here, and nothing branches on icons elsewhere.
 *
 * Uses static R references (not getIdentifier) so resource shrinking keeps the
 * drawables and the lookup stays fast.
 */
object KeyIcons {
    /** @return the drawable resource id, or 0 when no icon maps to [name]. */
    @DrawableRes
    fun resolve(name: String): Int = when (name) {
        // Keyboard special keys
        "shift" -> R.drawable.ic_key_shift
        "shift_fill" -> R.drawable.ic_key_shift_fill
        "shift_lock" -> R.drawable.ic_key_shift_lock
        "backspace" -> R.drawable.ic_key_backspace
        "keyboard_return" -> R.drawable.ic_key_enter
        "language" -> R.drawable.ic_key_language
        // Tone toolbar
        "text_fields" -> R.drawable.ic_tone_text_fields
        "chat_bubble_outline" -> R.drawable.ic_tone_chat
        "auto_awesome" -> R.drawable.ic_tone_auto_awesome
        "compress" -> R.drawable.ic_tone_compress
        "build" -> R.drawable.ic_tone_build
        "smart_toy" -> R.drawable.ic_tone_smart_toy
        "spellcheck" -> R.drawable.ic_tone_spellcheck
        // Voice toolbar
        "mic" -> R.drawable.ic_tone_mic
        else -> 0
    }
}
