namespace DraftRightWindows.Models;

/// <summary>
/// Category of a grammar-check finding. Kept as an enum with an explicit
/// wire mapping rather than comparing raw strings at each call site (Rule #1),
/// mirroring <see cref="Tone"/> and Linux's models/*.py.
/// </summary>
public enum GrammarIssueType
{
    Spelling,
    Grammar,
    Style,
    Other,
}

public static class GrammarIssueTypeExtensions
{
    /// <summary>
    /// Parse the backend's `type` string. Unknown values map to
    /// <see cref="GrammarIssueType.Other"/> rather than throwing — a new
    /// category added server-side must not break an older client.
    /// </summary>
    public static GrammarIssueType FromWire(string? wire) => wire?.ToLowerInvariant() switch
    {
        "spelling" => GrammarIssueType.Spelling,
        "grammar" => GrammarIssueType.Grammar,
        "style" => GrammarIssueType.Style,
        _ => GrammarIssueType.Other,
    };

    public static string WireValue(this GrammarIssueType type) => type switch
    {
        GrammarIssueType.Spelling => "spelling",
        GrammarIssueType.Grammar => "grammar",
        GrammarIssueType.Style => "style",
        _ => "other",
    };

    public static string DisplayName(this GrammarIssueType type) => type switch
    {
        GrammarIssueType.Spelling => "Spelling",
        GrammarIssueType.Grammar => "Grammar",
        GrammarIssueType.Style => "Style",
        _ => "Other",
    };

    /// <summary>
    /// Accent colour as a hex string, matching macOS GrammarCheckView:
    /// spelling = red, grammar = orange, style = blue. Returned as hex rather
    /// than a WinUI Color so this stays UI-framework-free and unit-testable.
    /// </summary>
    public static string HexColor(this GrammarIssueType type) => type switch
    {
        GrammarIssueType.Spelling => "#ef4444",
        GrammarIssueType.Grammar => "#f59e0b",
        GrammarIssueType.Style => "#5d87ff",
        _ => "#94a3b8",
    };
}
