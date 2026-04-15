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
