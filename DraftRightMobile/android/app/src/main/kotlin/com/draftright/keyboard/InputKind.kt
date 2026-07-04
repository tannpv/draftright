package com.draftright.keyboard

/** How the user produced the text. SPEECH adds a server-side cleanup preamble. */
enum class InputKind {
    TYPED, SPEECH;
    val apiValue: String get() = when (this) {
        TYPED -> "typed"
        SPEECH -> "speech"
    }
}
