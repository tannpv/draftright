namespace DraftRightWindows.Models;

/// <summary>
/// Top-level interaction mode for the rewrite feature.
/// </summary>
public enum AppMode
{
    /// <summary>Hotkey opens the diff panel; user picks a tone (legacy behavior).</summary>
    Advanced,

    /// <summary>Hotkey triggers an instant rewrite with the preset tone and pastes the result.</summary>
    OneClick
}

public static class AppModeExtensions
{
    /// <summary>Stable lowercase string used for JSON persistence (matches macOS/Linux).</summary>
    public static string ApiValue(this AppMode mode) => mode switch
    {
        AppMode.Advanced => "advanced",
        AppMode.OneClick => "oneClick",
        _ => "advanced"
    };

    public static AppMode FromApiValue(string? raw) => raw switch
    {
        "advanced" => AppMode.Advanced,
        "oneClick" => AppMode.OneClick,
        _ => AppMode.Advanced
    };

    public static string DisplayName(this AppMode mode) => mode switch
    {
        AppMode.Advanced => "Advanced",
        AppMode.OneClick => "One-Click",
        _ => "Advanced"
    };
}
