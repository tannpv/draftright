using DraftRightWindows.Services;
using Xunit;

namespace DraftRightWindows.Tests;

/// <summary>
/// Covers the admin-driven <see cref="DRLogger.MinLevel"/> threshold mapping —
/// the bit that decides, from the backend's client_log_level string, how much
/// each client writes. The actual file-write gating is a thin comparison on top
/// of this mapping.
/// </summary>
public class LoggerTests
{
    [Theory]
    [InlineData("off", DRLogger.Level.OFF)]
    [InlineData("OFF", DRLogger.Level.OFF)]
    [InlineData("errors", DRLogger.Level.ERROR)]
    [InlineData("error", DRLogger.Level.ERROR)]
    [InlineData("warnings", DRLogger.Level.WARN)]
    [InlineData("warning", DRLogger.Level.WARN)]
    [InlineData("warn", DRLogger.Level.WARN)]
    [InlineData("info", DRLogger.Level.INFO)]
    [InlineData(" Info ", DRLogger.Level.INFO)]
    [InlineData("", DRLogger.Level.INFO)]
    [InlineData(null, DRLogger.Level.INFO)]
    [InlineData("bogus", DRLogger.Level.INFO)]
    public void SetMinLevelFromServer_MapsServerStringToThreshold(string? server, DRLogger.Level expected)
    {
        try
        {
            DRLogger.SetMinLevelFromServer(server);
            Assert.Equal(expected, DRLogger.MinLevel);
        }
        finally
        {
            // Don't leak threshold state into other tests (statics are shared).
            DRLogger.MinLevel = DRLogger.Level.INFO;
        }
    }

    [Fact]
    public void MinLevel_OrdersInfoBelowWarnBelowErrorBelowOff()
    {
        Assert.True(DRLogger.Level.INFO < DRLogger.Level.WARN);
        Assert.True(DRLogger.Level.WARN < DRLogger.Level.ERROR);
        Assert.True(DRLogger.Level.ERROR < DRLogger.Level.OFF);
    }
}
