namespace DraftRightWindows.Models;

public enum Tone
{
    Simple,
    Natural,
    Polished,
    Concise,
    Technical,
    Claude,
    GrammarCheck,
    Translate
}

public static class ToneExtensions
{
    public static string ApiValue(this Tone tone) => tone switch
    {
        Tone.Simple => "simple",
        Tone.Natural => "natural",
        Tone.Polished => "polished",
        Tone.Concise => "concise",
        Tone.Technical => "technical",
        Tone.Claude => "claude",
        Tone.GrammarCheck => "grammar_check",
        Tone.Translate => "translate",
        _ => "polished"
    };

    public static string DisplayName(this Tone tone) => tone switch
    {
        Tone.Simple => "Simple",
        Tone.Natural => "Natural",
        Tone.Polished => "Polished",
        Tone.Concise => "Concise",
        Tone.Technical => "Technical",
        Tone.Claude => "Claude Style",
        Tone.GrammarCheck => "Grammar Check",
        Tone.Translate => "Translate",
        _ => "Polished"
    };

    public static string Icon(this Tone tone) => tone switch
    {
        Tone.Simple => "\u270e",
        Tone.Natural => "\U0001f4ac",
        Tone.Polished => "\u2728",
        Tone.Concise => "\u2296",
        Tone.Technical => "\U0001f527",
        Tone.Claude => "\U0001f916",
        Tone.GrammarCheck => "\u2713",
        Tone.Translate => "\U0001f310",
        _ => "\u2728"
    };
}
