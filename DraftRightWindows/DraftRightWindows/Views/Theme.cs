using System.Drawing;

namespace DraftRightWindows.Views;

/// <summary>
/// The dark palette every view shares. Values match the design tokens used by
/// the Linux app and the marketing site.
///
/// These RGB triples were previously retyped across nine view files — 44 local
/// declarations plus 13 inline literals. Every view now reads them from here.
/// Keep it that way: a palette scattered across call sites drifts one file at a
/// time and nobody notices until two dialogs are visibly different shades
/// (Rule #1, no hardcoding).
///
/// Colours used exactly once and carrying no shared meaning — a single status
/// banner's tint, say — stay inline at their call site. Naming a value used
/// once adds indirection without adding a source of truth.
/// </summary>
internal static class Theme
{
    public static readonly Color BgDark       = Color.FromArgb(15, 23, 42);
    public static readonly Color CardBg       = Color.FromArgb(30, 41, 59);
    public static readonly Color ResultBg     = Color.FromArgb(15, 41, 34);
    public static readonly Color BrandBlue    = Color.FromArgb(93, 135, 255);
    public static readonly Color TextPrimary  = Color.FromArgb(226, 232, 240);
    public static readonly Color TextMuted    = Color.FromArgb(148, 163, 184);
    public static readonly Color BorderColor  = Color.FromArgb(51, 65, 85);
    public static readonly Color ErrorRed     = Color.FromArgb(239, 68, 68);
    public static readonly Color SuccessGreen = Color.FromArgb(34, 197, 94);

    /// <summary>Background wash behind deleted words in the diff view.</summary>
    public static readonly Color DiffDeletedBg  = Color.FromArgb(94, 30, 40);

    /// <summary>Background wash behind inserted words in the diff view.</summary>
    public static readonly Color DiffInsertedBg = Color.FromArgb(20, 68, 48);

    /// <summary>
    /// The same colour as a WinUI brush.
    ///
    /// WinForms speaks <see cref="System.Drawing.Color"/> and WinUI speaks
    /// <c>Windows.UI.Color</c>. Bridging here means a WinUI surface reuses this
    /// palette rather than restating the hex values — the whole point of
    /// consolidating them (Rule #1). Delete this along with the WinForms views
    /// if the UI ever moves over wholesale.
    /// </summary>
    public static Microsoft.UI.Xaml.Media.SolidColorBrush Brush(Color c) =>
        new(Windows.UI.Color.FromArgb(c.A, c.R, c.G, c.B));
}
