using System.Drawing;
using System.Reflection;

namespace DraftRightWindows.Helpers;

/// <summary>
/// Loads the DraftRight application icon for window / taskbar use.
///
/// The .ico is read from an embedded assembly resource rather than from disk.
/// Single-file publish (PublishSingleFile + IncludeAllContentForSelfExtract)
/// bundles Content into the exe and extracts it to AppContext.BaseDirectory —
/// NOT next to Environment.ProcessPath. The old
/// Path.Combine(Path.GetDirectoryName(ProcessPath), "Assets", "DraftRight.ico")
/// + File.Exists check therefore always missed in shipped single-file builds,
/// so windows fell back to the generic taskbar icon (BUG-47 / #78). An embedded
/// resource has no on-disk dependency and resolves identically in single-file,
/// framework-dependent, and MSIX builds.
/// </summary>
public static class AppIcon
{
    // Pinned via <LogicalName> in the .csproj so this name is stable regardless
    // of the Assets/ folder path or root namespace.
    private const string ResourceName = "DraftRight.ico";

    private static Icon? _cached;
    private static bool _loaded;

    /// <summary>
    /// The app icon, or null if it can't be loaded. Best-effort: callers should
    /// tolerate null and leave the window's default icon in place. The returned
    /// Icon is a shared cached instance — do not dispose it.
    /// </summary>
    public static Icon? Load()
    {
        if (_loaded) return _cached;
        _loaded = true;
        try
        {
            using var stream = Assembly.GetExecutingAssembly().GetManifestResourceStream(ResourceName);
            if (stream != null) _cached = new Icon(stream);
        }
        catch { /* best-effort — fall back to the default icon */ }
        return _cached;
    }
}
