using DraftRightWindows.Models;
using Xunit;

namespace DraftRightWindows.PureTests;

/// <summary>
/// The trigger-mode enum — pencil or hotkey, mutually exclusive. Its ApiValue
/// strings must stay byte-identical to macOS/Linux for settings.json parity
/// (DraftRight #180).
/// </summary>
public class TriggerModeTests
{
    [Fact]
    public void Pencil_UsesPencilOnly()
    {
        Assert.True(TriggerMode.Pencil.UsesPencil());
        Assert.False(TriggerMode.Pencil.UsesHotkey());
    }

    [Fact]
    public void Hotkey_UsesHotkeyOnly()
    {
        Assert.False(TriggerMode.Hotkey.UsesPencil());
        Assert.True(TriggerMode.Hotkey.UsesHotkey());
    }

    [Theory]
    [InlineData(TriggerMode.Pencil, "pencil")]
    [InlineData(TriggerMode.Hotkey, "hotkey")]
    public void ApiValue_IsStableAndCrossPlatform(TriggerMode mode, string expected)
    {
        Assert.Equal(expected, mode.ApiValue());
    }

    [Theory]
    [InlineData("pencil", TriggerMode.Pencil)]
    [InlineData("hotkey", TriggerMode.Hotkey)]
    [InlineData("both", TriggerMode.Hotkey)]   // removed mode migrates to the default
    [InlineData(null, TriggerMode.Hotkey)]
    [InlineData("garbage", TriggerMode.Hotkey)]
    public void FromApiValue_ParsesAndMigrates(string? raw, TriggerMode expected)
    {
        Assert.Equal(expected, TriggerModeExtensions.FromApiValue(raw));
    }
}
