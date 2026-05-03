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

    // Use BMP-only glyphs so Segoe UI renders them directly without falling
    // back to Segoe UI Emoji (which often shows as tofu boxes in WinForms).
    public static string Icon(this Tone tone) => tone switch
    {
        Tone.Simple       => "\u270e",  // \u270e pencil
        Tone.Natural      => "\u275d",  // \u275d heavy double turned comma quotation
        Tone.Polished     => "\u2728",  // \u2728 sparkles (renders fine in Segoe UI)
        Tone.Concise      => "\u2296",  // \u2296 circled minus
        Tone.Technical    => "\u2699",  // \u2699 gear
        Tone.Claude       => "\u24b8",  // \u24b8 circled C
        Tone.GrammarCheck => "\u2713",  // \u2713 check mark
        Tone.Translate    => "\u21c4",  // \u21c4 rightwards arrow over leftwards
        _ => "\u2728"
    };
}
