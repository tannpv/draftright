using System;
using System.Drawing;
using System.IO;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows.Services;

/// <summary>
/// Primary-display screen capture + downscale, shared by every surface that
/// attaches a screenshot. Extracted from the WinForms ReportBugDialog during
/// the WPF migration (#158) so the WPF dialog and the phase-4 Suggest-a-feature
/// dialog use one implementation instead of copies — the two tuning constants
/// and the capture/downscale logic have a single source of truth (RULE #1).
///
/// System.Drawing is used (not WPF imaging) because capture is inherently GDI
/// and the result is saved to a PNG the multipart upload streams from disk;
/// callers that need a WPF preview load that PNG into a BitmapImage.
/// </summary>
internal static class ScreenCapture
{
    /// <summary>
    /// Largest edge a captured screenshot keeps, in pixels. A full-screen PNG
    /// off a 4K display runs well past the 5 MB upload cap
    /// (<see cref="BugReportService.MaxScreenshotBytes"/>) and would only fail
    /// at submit time, so captures are downscaled first. Matches the macOS
    /// sheet, which uses the same bound for the same reason.
    /// </summary>
    public const int MaxDimension = 2000;

    /// <summary>
    /// Pause between hiding the requesting window and grabbing the screen, so
    /// the compositor has actually removed that window from the frame. macOS
    /// uses the same 250 ms.
    /// </summary>
    public const int HideDelayMs = 250;

    /// <summary>
    /// Captures the primary display, downscales it to <see cref="MaxDimension"/>,
    /// and writes it to a temp PNG the caller owns (delete when done). Returns
    /// the temp file path.
    /// </summary>
    public static string CaptureToTempPng()
    {
        using var shot = CapturePrimary();
        using var scaled = Downscale(shot, MaxDimension);
        var tmp = Path.Combine(Path.GetTempPath(), $"draftright-capture-{Guid.NewGuid():N}.png");
        scaled.Save(tmp, System.Drawing.Imaging.ImageFormat.Png);
        return tmp;
    }

    /// <summary>
    /// Grabs the primary display. Matches macOS, which captures the main
    /// display rather than every monitor — a multi-monitor grab produces an
    /// enormous, mostly-empty image that is harder to read, not easier.
    /// </summary>
    public static Bitmap CapturePrimary()
    {
        var bounds = WinForms.Screen.PrimaryScreen?.Bounds
            ?? throw new InvalidOperationException("No primary display found.");
        var bmp = new Bitmap(bounds.Width, bounds.Height);
        try
        {
            using var g = Graphics.FromImage(bmp);
            g.CopyFromScreen(bounds.Location, Point.Empty, bounds.Size);
            return bmp;
        }
        catch
        {
            bmp.Dispose();
            throw;
        }
    }

    /// <summary>
    /// Shrinks <paramref name="src"/> so its longest edge is at most
    /// <paramref name="maxDimension"/>, preserving aspect ratio. Returns a copy
    /// either way, so the caller can dispose both without special cases.
    /// </summary>
    public static Bitmap Downscale(Image src, int maxDimension)
    {
        var scale = Math.Min(1.0, (double)maxDimension / Math.Max(src.Width, src.Height));
        var w = Math.Max(1, (int)Math.Round(src.Width * scale));
        var h = Math.Max(1, (int)Math.Round(src.Height * scale));

        var outBmp = new Bitmap(w, h);
        try
        {
            using var g = Graphics.FromImage(outBmp);
            g.InterpolationMode = System.Drawing.Drawing2D.InterpolationMode.HighQualityBicubic;
            g.DrawImage(src, 0, 0, w, h);
            return outBmp;
        }
        catch
        {
            outBmp.Dispose();
            throw;
        }
    }
}
