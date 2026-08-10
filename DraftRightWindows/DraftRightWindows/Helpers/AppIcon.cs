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

    /// <summary>
    /// The app icon as a WPF <see cref="System.Windows.Media.ImageSource"/> for
    /// <c>Window.Icon</c>, or null if it can't be loaded. Reads the SAME embedded
    /// <see cref="ResourceName"/> resource as <see cref="Load"/> — one source of
    /// truth — so WinForms and WPF windows show the identical icon.
    ///
    /// Deliberately NOT cached/shared. Every WPF window opens on its own STA
    /// thread, and an ImageSource is owned by the thread that created it:
    /// <c>Window.Show()</c> → <c>UpdateIcon</c> reads the image's frames and
    /// throws "the calling thread cannot access this object" if the icon was
    /// built on another thread. That crashed the app when a second window opened
    /// on a different thread from the first. Decoding fresh per call — into a
    /// frozen, decoder-detached <see cref="System.Windows.Media.Imaging.BitmapImage"/>
    /// (OnLoad) on the CALLING thread — keeps each window's icon on its own thread.
    /// </summary>
    public static System.Windows.Media.ImageSource? LoadImageSource()
    {
        try
        {
            using var stream = Assembly.GetExecutingAssembly().GetManifestResourceStream(ResourceName);
            if (stream == null) return null;
            var bmp = new System.Windows.Media.Imaging.BitmapImage();
            bmp.BeginInit();
            bmp.CacheOption = System.Windows.Media.Imaging.BitmapCacheOption.OnLoad;
            bmp.StreamSource = stream;
            bmp.EndInit();
            bmp.Freeze();
            return bmp;
        }
        catch { return null; /* best-effort — fall back to the default icon */ }
    }
}
