package com.draftright.keyboard

class KeyboardController(
    private val registry: LanguageRegistry,
    enabledIds: List<String>,
    activeId: String,
) {
    var enabled: List<LanguagePack> =
        registry.all.filter { it.id in enabledIds }
            .ifEmpty { listOf(registry.byIdOrDefault("en")) }
        private set

    var current: LanguagePack =
        enabled.firstOrNull { it.id == activeId } ?: enabled.first()
        private set

    var composer: Composer? = current.composer()
        private set

    fun cycleLanguage() {
        if (enabled.size <= 1) return
        val idx = enabled.indexOfFirst { it.id == current.id }
        val nextIdx = if (idx < 0) 0 else (idx + 1) % enabled.size
        composer?.reset()
        current = enabled[nextIdx]
        composer = current.composer()
    }

    fun setActive(id: String) {
        val target = enabled.firstOrNull { it.id == id } ?: return
        if (target.id == current.id) return
        composer?.reset()
        current = target
        composer = current.composer()
    }
}
