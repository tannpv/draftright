package com.draftright.keyboard

class KeyboardController(
    private val registry: LanguageRegistry,
    enabledIds: List<String>,
    activeId: String,
    /** When true, Japanese uses the flick kana composer instead of rōmaji (#212). */
    private val jpFlick: Boolean = false,
) {
    var enabled: List<LanguagePack> =
        registry.all.filter { it.id in enabledIds }
            .ifEmpty { listOf(registry.byIdOrDefault("en")) }
        private set

    var current: LanguagePack =
        enabled.firstOrNull { it.id == activeId } ?: enabled.first()
        private set

    var composer: Composer? = makeComposer(current)
        private set

    /** Japanese + flick mode gets the kana passthrough composer; every other case
     *  keeps the pack's own composer, so non-flick behaviour is unchanged. */
    private fun makeComposer(pack: LanguagePack): Composer? =
        if (pack.id == "ja" && jpFlick) com.draftright.keyboard.composer.KanaComposer() else pack.composer()

    fun cycleLanguage(reverse: Boolean = false) {
        if (enabled.size <= 1) return
        val idx = enabled.indexOfFirst { it.id == current.id }
        val step = if (reverse) -1 else 1
        val rawIdx = (if (idx < 0) 0 else idx) + step
        val nextIdx = ((rawIdx % enabled.size) + enabled.size) % enabled.size
        composer?.reset()
        current = enabled[nextIdx]
        composer = makeComposer(current)
    }

    fun setActive(id: String) {
        val target = enabled.firstOrNull { it.id == id } ?: return
        if (target.id == current.id) return
        composer?.reset()
        current = target
        composer = makeComposer(current)
    }

    fun onKey(char: Char): KeystrokeOutcome {
        val c = composer ?: return KeystrokeOutcome.Commit(char.toString())
        return when (val r = c.onKey(char)) {
            is ComposeResult.Commit -> KeystrokeOutcome.Commit(r.text)
            is ComposeResult.Composing -> KeystrokeOutcome.Composing(r.text)
            ComposeResult.PassThrough -> KeystrokeOutcome.Commit(char.toString())
            ComposeResult.Consumed -> KeystrokeOutcome.NoChange
        }
    }

    fun onBackspace(): KeystrokeOutcome {
        val c = composer ?: return KeystrokeOutcome.DeleteOne
        return when (val r = c.onBackspace()) {
            is ComposeResult.Composing -> KeystrokeOutcome.Composing(r.text)
            ComposeResult.Consumed -> KeystrokeOutcome.NoChange
            ComposeResult.PassThrough -> KeystrokeOutcome.DeleteOne
            is ComposeResult.Commit -> KeystrokeOutcome.Commit(r.text)
        }
    }
}

sealed class KeystrokeOutcome {
    data class Commit(val text: String) : KeystrokeOutcome()
    data class Composing(val text: String) : KeystrokeOutcome()
    object DeleteOne : KeystrokeOutcome()
    object NoChange : KeystrokeOutcome()
}
