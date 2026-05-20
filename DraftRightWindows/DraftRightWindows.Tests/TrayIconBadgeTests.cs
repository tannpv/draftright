using System.Drawing;
using DraftRightWindows.Services;
using Xunit;

namespace DraftRightWindows.Tests;

/// <summary>
/// Guards the tray "update ready" badge compositing — its real risk is a GDI /
/// native-handle exception, so these assert it produces a usable, correctly
/// sized Icon without throwing or leaking the source handle.
/// </summary>
public class TrayIconBadgeTests
{
    [Fact]
    public void WithDot_ReturnsUsableIcon_MatchingBaseSize()
    {
        using var baseIcon = SystemIcons.Application;

        using var badged = TrayIconBadge.WithDot(baseIcon);

        Assert.NotNull(badged);
        // Badge is composited at the base icon's pixel size.
        Assert.Equal(baseIcon.Size, badged.Size);
        // The clone is independent of any native handle — turning it back into
        // a bitmap must succeed (would throw if the HICON had been destroyed
        // out from under it).
        using var bmp = badged.ToBitmap();
        Assert.True(bmp.Width > 0 && bmp.Height > 0);
    }

    [Fact]
    public void WithDot_AcceptsCustomColor()
    {
        using var baseIcon = SystemIcons.Information;

        using var badged = TrayIconBadge.WithDot(baseIcon, Color.Orange);

        Assert.NotNull(badged);
    }
}
