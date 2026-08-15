using DraftRightWindows.Services;
using Xunit;

namespace DraftRightWindows.PureTests;

/// <summary>
/// Pure manifest logic: version comparison, per-platform normalization, and
/// artifact-hash resolution. Moved out of DraftRightWindows.Tests (issue #80)
/// so these run headlessly — they exercise <see cref="UpdateManifest"/>
/// directly and never load the WinUI assembly.
///
/// The instance-level UpdateService tests (HTTP retry, staging, install) stay
/// in DraftRightWindows.Tests: they construct UpdateService, which pulls in
/// WinForms, so they remain a Windows-only suite.
/// </summary>
public class UpdateManifestTests
{
    private const string Windows = "windows";

    // ── IsNewer: pure version-compare logic ─────────────────────────────────

    [Theory]
    [InlineData("2.2.5", "2.2.4", true)]
    [InlineData("2.2.4", "2.2.4", false)]
    [InlineData("2.2.4", "2.2.5", false)]
    [InlineData("2.10.0", "2.9.0", true)]   // numeric, not lex (would fail under string compare: "10" < "9")
    [InlineData("3.0.0", "2.99.0", true)]
    [InlineData("2.2.0", "2.2.0.0", false)] // missing component treated as 0
    [InlineData("2.2.0.1", "2.2.0", true)]  // longer wins when extra component > 0
    [InlineData("", "2.2.0", false)]        // garbage → zeros → not newer
    public void IsNewer_Numeric_AndPadding(string remote, string local, bool expected)
    {
        Assert.Equal(expected, UpdateManifest.IsNewer(remote, local));
    }

    // ── NormalizeForPlatform: the platform pin is authoritative ─────────────
    //
    // Regression: 2.2.10 users got stuck in a "current 2.2.10, install 2.3.1,
    // still 2.2.10" loop because the backend's top-level `version` is a
    // cross-platform max (2.3.1 from mac) but `windows_url` is the Windows
    // row's URL (pointing at the 2.2.10 installer). The client must read
    // `platforms.windows` as the authoritative source so it can never be
    // tricked into installing the wrong-versioned installer.

    [Fact]
    public void Normalize_PrefersPlatformPin_OverLegacyTopLevel()
    {
        var raw = new UpdateInfo
        {
            Version = "2.3.1",                                          // cross-platform max — bogus for windows
            WindowsUrl = "https://x/installer-2.2.10.exe",              // actually a 2.2.10 installer
            ReleaseNotes = "mac notes",
            Platforms = new()
            {
                ["windows"] = new PlatformRelease
                {
                    Version = "2.2.10",
                    Url = "https://x/installer-2.2.10.exe",
                    Notes = "windows-specific notes",
                    Required = false,
                },
                ["mac"] = new PlatformRelease { Version = "2.3.1", Url = "https://x/mac.dmg" },
            },
        };

        var n = UpdateManifest.NormalizeForPlatform(raw, Windows);

        Assert.Equal("2.2.10", n.Version);
        Assert.Equal("https://x/installer-2.2.10.exe", n.WindowsUrl);
        Assert.Equal("windows-specific notes", n.ReleaseNotes);
    }

    [Fact]
    public void Normalize_FallsThrough_WhenPlatformsMapAbsent_ForLegacyBackends()
    {
        var raw = new UpdateInfo
        {
            Version = "2.2.5",
            WindowsUrl = "https://x/old.exe",
            ReleaseNotes = "legacy",
        };

        var n = UpdateManifest.NormalizeForPlatform(raw, Windows);

        Assert.Equal("2.2.5", n.Version);
        Assert.Equal("https://x/old.exe", n.WindowsUrl);
        Assert.Equal("legacy", n.ReleaseNotes);
    }

    [Fact]
    public void Normalize_FallsThrough_WhenPlatformEntryHasEmptyVersion()
    {
        // Defensive: a half-populated row shouldn't blank out a valid top-level.
        var raw = new UpdateInfo
        {
            Version = "2.2.5",
            WindowsUrl = "https://x/installer-2.2.5.exe",
            Platforms = new()
            {
                ["windows"] = new PlatformRelease { Version = "", Url = "" },
            },
        };

        var n = UpdateManifest.NormalizeForPlatform(raw, Windows);

        Assert.Equal("2.2.5", n.Version);
        Assert.Equal("https://x/installer-2.2.5.exe", n.WindowsUrl);
    }

    // ── ResolveSha256: per-platform pin wins, lowercased ────────────────────

    [Fact]
    public void ResolveSha256_PrefersPlatformEntry_Lowercased()
    {
        var info = new UpdateInfo
        {
            WindowsSha256 = "AAAA",
            Platforms = new()
            {
                ["windows"] = new PlatformRelease { Version = "2.4.1", Url = "https://x/a.exe", Sha256 = "BBBB" },
            },
        };
        Assert.Equal("bbbb", UpdateManifest.ResolveSha256(info, Windows));
    }

    [Fact]
    public void ResolveSha256_FallsBackToTopLevel_WhenNoPlatformHash()
    {
        var info = new UpdateInfo { WindowsSha256 = "CcCc" };
        Assert.Equal("cccc", UpdateManifest.ResolveSha256(info, Windows));
    }

    [Fact]
    public void ResolveSha256_EmptyWhenUnpublished()
    {
        // Empty means "publisher shipped no hash" — callers install with a
        // warning rather than hard-failing, for releases predating #22.
        Assert.Equal("", UpdateManifest.ResolveSha256(new UpdateInfo(), Windows));
    }

    [Fact]
    public void Normalize_CarriesWindowsSha256FromPin()
    {
        var raw = new UpdateInfo
        {
            Version = "2.3.1",
            Platforms = new()
            {
                ["windows"] = new PlatformRelease { Version = "2.4.1", Url = "https://x/a.exe", Sha256 = "deadbeef" },
            },
        };
        var n = UpdateManifest.NormalizeForPlatform(raw, Windows);
        Assert.Equal("deadbeef", n.WindowsSha256);
    }

    // ── UpdateLabel: the wording every surface shows ────────────────────────
    //
    // The version is optional. The Store backend has no trustworthy one to
    // give (StorePackageUpdate.Package.Id.Version reports the package being
    // replaced, not the one on offer), so it passes none and the label must
    // degrade cleanly rather than leaving a gap where a number should be.

    [Theory]
    [InlineData("2.3.63", false, "Update 2.3.63 available — install now")]
    [InlineData("2.3.63", true, "Update 2.3.63 ready — restart & install")]
    [InlineData(null, false, "Update available — install now")]
    [InlineData("", true, "Update ready — restart & install")]
    [InlineData("   ", false, "Update available — install now")] // whitespace is not a version
    public void Tray_OmitsVersionWhenUnknown(string? version, bool staged, string expected)
    {
        Assert.Equal(expected, UpdateLabel.Tray(version, staged));
    }

    [Theory]
    [InlineData("2.3.63", false, "Update 2.3.63 available — click to download and install")]
    [InlineData("2.3.63", true, "Update 2.3.63 downloaded — click to restart and install")]
    [InlineData(null, false, "Update available — click to download and install")]
    [InlineData("", true, "Update downloaded — click to restart and install")]
    public void Settings_OmitsVersionWhenUnknown(string? version, bool staged, string expected)
    {
        Assert.Equal(expected, UpdateLabel.Settings(version, staged));
    }

    [Fact]
    public void Labels_NeverContainDoubleSpace()
    {
        // The bug this guards: interpolating an empty version straight into
        // "Update {version} available" rendered "Update  available".
        foreach (var staged in new[] { true, false })
        {
            Assert.DoesNotContain("  ", UpdateLabel.Tray("", staged));
            Assert.DoesNotContain("  ", UpdateLabel.Settings(null, staged));
        }
    }
}
