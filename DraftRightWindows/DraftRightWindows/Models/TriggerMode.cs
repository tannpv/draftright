namespace DraftRightWindows.Models;

/// <summary>
/// How a rewrite is triggered — the pencil button on selection, or a global
/// hotkey. Mutually exclusive: exactly one is active at a time. Single source of
/// truth, decoupled from whether a hotkey combo is set (so switching to the
/// pencil keeps the saved shortcut). Mirrors the macOS <c>TriggerMode</c>
/// (DraftRight #180).
/// </summary>
public enum TriggerMode
{
    /// <summary>The pencil button appears when text is highlighted by dragging.</summary>
    Pencil,

    /// <summary>A global keyboard shortcut opens the panel.</summary>
    Hotkey
}

public static class TriggerModeExtensions
{
    /// <summary>Whether the on-selection pencil should be active in this mode.</summary>
    public static bool UsesPencil(this TriggerMode mode) => mode == TriggerMode.Pencil;

    /// <summary>Whether the global hotkey should be registered in this mode.</summary>
    public static bool UsesHotkey(this TriggerMode mode) => mode == TriggerMode.Hotkey;

    /// <summary>
    /// Stable lowercase string used for JSON persistence. MUST stay byte-identical
    /// to macOS <c>TriggerMode.rawValue</c> and the Linux equivalent so the three
    /// platforms' <c>settings.json</c> stays interoperable (same discipline as
    /// <see cref="AppModeExtensions.ApiValue"/>).
    /// </summary>
    public static string ApiValue(this TriggerMode mode) => mode switch
    {
        TriggerMode.Pencil => "pencil",
        TriggerMode.Hotkey => "hotkey",
        _ => "hotkey"
    };

    public static TriggerMode FromApiValue(string? raw) => raw switch
    {
        "pencil" => TriggerMode.Pencil,
        "hotkey" => TriggerMode.Hotkey,
        // "both" was removed (#180); a stored value migrates to the hotkey default.
        _ => TriggerMode.Hotkey
    };

    public static string DisplayName(this TriggerMode mode) => mode switch
    {
        TriggerMode.Pencil => "Pencil",
        TriggerMode.Hotkey => "Hotkey",
        _ => "Hotkey"
    };
}
