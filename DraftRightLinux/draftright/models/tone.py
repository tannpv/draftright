from enum import Enum


class Tone(Enum):
    SIMPLE = ("simple", "Simple", "\u270e", "Easy language, short sentences")
    NATURAL = ("natural", "Natural", "\U0001f4ac", "Conversational, flows smoothly")
    POLISHED = ("polished", "Polished", "\u2728", "Professional, refined")
    CONCISE = ("concise", "Concise", "\u2296", "Removes redundancy")
    TECHNICAL = ("technical", "Technical", "\U0001f527", "Precise, documentation style")
    CLAUDE = ("claude", "Claude Style", "\U0001f916", "Clear, thoughtful, well-structured")
    GRAMMAR_CHECK = ("grammar_check", "Grammar Check", "\u2713", "Check spelling, grammar, style")
    TRANSLATE = ("translate", "Translate", "\U0001f310", "Translate to another language")

    def __init__(self, api_value, display_name, icon, description):
        self.api_value = api_value
        self.display_name = display_name
        self.icon = icon
        self.description = description

    @property
    def uses_target_language(self) -> bool:
        """True when the result depends on the configured translate language.

        Drives both what is sent to /rewrite and what goes into the cache key
        — without it, switching language would serve the previous language's
        translation from cache.
        """
        return self is Tone.TRANSLATE

    @property
    def returns_grammar_analysis(self) -> bool:
        """True when /rewrite answers with a ``grammar`` object, not text.

        Grammar Check is the one tone whose response has no ``rewritten_text``
        — parsing it as one raised "/rewrite response has no rewritten_text"
        and the tone did nothing but show an error (#107).
        """
        return self is Tone.GRAMMAR_CHECK

    @property
    def produces_replacement_text(self) -> bool:
        """True when the result can be pasted straight over the selection.

        One-Click has no UI to review anything, so it can only offer tones that
        yield replacement text — a grammar analysis has nothing to inject.
        """
        return not self.returns_grammar_analysis

    @classmethod
    def from_api_value(cls, value: str) -> "Tone | None":
        for tone in cls:
            if tone.api_value == value:
                return tone
        return None
