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

    /// <summary>Parse a backend API value (e.g. "polished", "grammar_check")
    /// back into a <see cref="Tone"/>, or null if it doesn't match a known
    /// tone. Inverse of <see cref="ApiValue"/>.</summary>
    public static Tone? FromApiValue(string? apiValue) => apiValue switch
    {
        "simple" => Tone.Simple,
        "natural" => Tone.Natural,
        "polished" => Tone.Polished,
        "concise" => Tone.Concise,
        "technical" => Tone.Technical,
        "claude" => Tone.Claude,
        "grammar_check" => Tone.GrammarCheck,
        "translate" => Tone.Translate,
        _ => null
    };

    /// <summary>
    /// True when the rewrite result depends on the configured target language,
    /// so callers must send it to the backend and fold it into the cache key.
    /// Mirrors Linux's <c>Tone.uses_target_language</c>. Kept on the enum so a
    /// future language-aware tone is one line here rather than another
    /// `tone == Tone.Translate` literal scattered through the UI (Rule #1).
    /// </summary>
    public static bool UsesTargetLanguage(this Tone tone) => tone == Tone.Translate;

    /// <summary>
    /// True when the tone's result is plain text that can be cached as a string.
    /// False for tones returning a structured payload — caching those as a
    /// string would silently drop the structure (Grammar Check's issues list).
    ///
    /// Must gate the cache read AND the cache write. Reading with one condition
    /// and writing with another produces entries that can never be hit but
    /// still consume the bounded cache — the same asymmetry that made Linux's
    /// translate results permanently un-cacheable (#108).
    /// </summary>
    public static bool ProducesCacheableText(this Tone tone) => tone != Tone.GrammarCheck;

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
