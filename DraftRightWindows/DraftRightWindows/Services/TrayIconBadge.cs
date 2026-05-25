using System;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.Runtime.InteropServices;

namespace DraftRightWindows.Services;

/// <summary>
/// Builds a copy of a tray icon with a small badge dot composited into the
/// bottom-right corner — used to flag "an update is ready to install" on the
/// always-visible system-tray icon (this is a tray app; the taskbar button
/// only exists while a window is open, so it's not a reliable surface).
///
/// Unpackaged WinUI app, so UWP tile/BadgeUpdateManager badges aren't
/// available; compositing onto the NotifyIcon image is the portable approach.
/// </summary>
public static class TrayIconBadge
{
    [DllImport("user32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool DestroyIcon(IntPtr hIcon);

    /// <summary>
    /// Returns a new <see cref="Icon"/> = <paramref name="baseIcon"/> with a
    /// filled dot in the bottom-right corner. The result is independent of any
    /// native handle (cloned), so the caller just disposes it like any Icon.
    /// </summary>
    public static Icon WithDot(Icon baseIcon, Color? dotColor = null)
    {
        var color = dotColor ?? Color.FromArgb(239, 68, 68); // red-500

        using var bmp = baseIcon.ToBitmap();
        int w = bmp.Width, h = bmp.Height;
        // Dot ~45% of the icon, kept inside the canvas with a 1px margin.
        int d = Math.Max(6, (int)(Math.Min(w, h) * 0.45));
        int x = w - d - 1;
        int y = h - d - 1;

        using (var g = Graphics.FromImage(bmp))
        {
            g.SmoothingMode = SmoothingMode.AntiAlias;
            // A contrasting ring so the dot reads on both light and dark icons.
            using var ring = new SolidBrush(Color.White);
            g.FillEllipse(ring, x - 1, y - 1, d + 2, d + 2);
            using var fill = new SolidBrush(color);
            g.FillEllipse(fill, x, y, d, d);
        }

        var hicon = bmp.GetHicon();
        try
        {
            using var tmp = Icon.FromHandle(hicon);
            return (Icon)tmp.Clone();
        }
        finally
        {
            DestroyIcon(hicon);
        }
    }
}
