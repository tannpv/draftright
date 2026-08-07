using System.Drawing;

namespace DraftRightWindows.Views;

/// <summary>
/// The dark palette every view shares. Values match the design tokens used by
/// the Linux app and the marketing site.
///
/// These RGB triples were previously retyped in nine separate view files. Any
/// new view must read them from here rather than adding a tenth copy — a
/// palette scattered across call sites drifts one file at a time and nobody
/// notices until two dialogs are visibly different shades (Rule #1).
///
/// Existing views still carry their own copies; migrating them is a mechanical
/// change kept out of the feature commit that introduced this file.
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
}
