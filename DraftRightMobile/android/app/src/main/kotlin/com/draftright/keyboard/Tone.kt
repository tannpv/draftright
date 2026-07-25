package com.draftright.keyboard

enum class Tone(
    val displayName: String,
    val iconRes: String // Material icon name reference
) {
    SIMPLE("Simple", "text_fields"),
    NATURAL("More Natural", "chat_bubble_outline"),
    POLISHED("More Polished", "auto_awesome"),
    CONCISE("Concise", "compress"),
    TECHNICAL("Technical", "build"),
    CLAUDE("Claude Style", "smart_toy"),
    GRAMMAR_CHECK("Grammar Check", "spellcheck"),
    TRANSLATE("Translate", "language");

    val apiValue: String get() = when (this) {
        SIMPLE -> "simple"
        NATURAL -> "natural"
        POLISHED -> "polished"
        CONCISE -> "concise"
        TECHNICAL -> "technical"
        CLAUDE -> "claude"
        GRAMMAR_CHECK -> "grammar_check"
        TRANSLATE -> "translate"
    }

    fun systemPrompt(targetLanguage: String = "English"): String = when (this) {
        SIMPLE -> "Rewrite the following text using simple, easy-to-understand language. Use short sentences and common words. Preserve the original meaning. Return only the rewritten text, no explanations."
        NATURAL -> "Rewrite the following text to sound more natural and conversational, as if spoken by a real person. Remove awkward phrasing and make it flow smoothly. Preserve the original meaning. Return only the rewritten text, no explanations."
        POLISHED -> "Rewrite the following text to be more polished and professional. Improve grammar, word choice, and sentence structure for a refined, workplace-appropriate tone. Preserve the original meaning. Return only the rewritten text, no explanations."
        CONCISE -> "Rewrite the following text to be as concise as possible. Remove unnecessary words, redundancy, and filler while preserving the key meaning. Return only the rewritten text, no explanations."
        TECHNICAL -> "Rewrite the following text in a technical specification style. Use precise, unambiguous language suitable for documentation, specs, or technical communication. Preserve the original meaning. Return only the rewritten text, no explanations."
        CLAUDE -> "Rewrite the following text in a clear, thoughtful, and well-structured style. Be direct but warm — every sentence should carry weight. Use good paragraph breaks and logical flow. Acknowledge nuance where relevant without over-hedging. Sound naturally confident and approachable, not formal or stiff. Preserve the original meaning. Return only the rewritten text, no explanations."
        GRAMMAR_CHECK -> "Analyze the given text for grammar, spelling, and style issues. Return a JSON object with a \"score\" (0-100) and an \"issues\" array. Each issue has \"type\", \"offset\", \"length\", \"original\", \"suggestion\", and \"reason\". Return ONLY JSON."
        TRANSLATE -> "Translate the following text into $targetLanguage. If the text is already in $targetLanguage, translate it into English instead. Preserve the original meaning and tone. Return only the translated text, no explanations."
    }

    companion object {
        /** Resolve a persisted [apiValue] back to its Tone, or null if unknown.
         *  Lets settings store the stable api string instead of an ordinal. */
        fun fromApiValue(value: String?): Tone? =
            value?.let { v -> entries.firstOrNull { it.apiValue == v } }
    }
}
