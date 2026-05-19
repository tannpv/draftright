using System;
using System.Net;
using System.Net.Http;
using System.Threading.Tasks;
using DraftRightWindows.Services;
using Xunit;

namespace DraftRightWindows.Tests;

/// <summary>
/// Unit tests for <see cref="UpdateService"/>. Covers the failure modes that
/// hit users in the wild — version comparison edge cases, "no update" / "newer
/// available" / "newer-but-missing-URL" classification, and retry behavior on
/// transient download failures (the kind of regression that would re-introduce
/// the 2.2.3 "Updating DraftRight" hang).
///
/// HTTP injection via <see cref="TestHttpHandler"/>; ConnectTimeout itself
/// (a SocketsHttpHandler property) isn't exercised here — that's a behavior
/// of real network I/O, validated by manual smoke against a blackhole IP.
/// </summary>
public class UpdateServiceTests
{
    private const string BackendUrl = "https://api.example.test";

    private static UpdateService BuildService(string currentVersion, TestHttpHandler metadataHandler, TestHttpHandler downloadHandler)
    {
        var http = new HttpClient(metadataHandler) { Timeout = TimeSpan.FromSeconds(10) };
        var download = new HttpClient(downloadHandler) { Timeout = TimeSpan.FromMinutes(1) };
        return new UpdateService(currentVersion, BackendUrl, http, download);
    }

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
        Assert.Equal(expected, UpdateService.IsNewerForTest(remote, local));
    }

    // ── RefreshAvailableUpdateAsync: classification ─────────────────────────

    [Fact]
    public async Task Refresh_ReturnsNull_WhenServerVersionEqualsCurrent()
    {
        var meta = TestHttpHandler.Always(HttpStatusCode.OK,
            """{"version":"2.2.4","windows_url":"https://x/inst.exe"}""");
        // Download handler should never be called — but provide a safe default.
        var dl = TestHttpHandler.Bytes(new byte[] { 0x4D, 0x5A });
        var svc = BuildService("2.2.4", meta, dl);

        var result = await svc.RefreshAvailableUpdateAsync();

        Assert.Null(result);
        Assert.Null(svc.AvailableUpdate);
    }

    [Fact]
    public async Task Refresh_ReturnsNull_WhenWindowsUrlIsEmpty_EvenIfNewer()
    {
        var meta = TestHttpHandler.Always(HttpStatusCode.OK,
            """{"version":"2.2.5","windows_url":""}""");
        var dl = TestHttpHandler.Bytes(new byte[] { 0x4D, 0x5A });
        var svc = BuildService("2.2.4", meta, dl);

        var result = await svc.RefreshAvailableUpdateAsync();

        Assert.Null(result);
        Assert.Null(svc.AvailableUpdate);
    }

    [Fact]
    public async Task Refresh_ReturnsInfo_AndSetsAvailableUpdate_WhenNewer()
    {
        var meta = TestHttpHandler.Always(HttpStatusCode.OK,
            """{"version":"2.2.5","windows_url":"https://x/installer-2.2.5.exe","release_notes":"hardened updater"}""");
        // Provide a tiny valid "installer" so the background staging task succeeds quickly.
        var dl = TestHttpHandler.Bytes(new byte[] { 0x4D, 0x5A, 0x90, 0x00 });
        var svc = BuildService("2.2.4", meta, dl);

        var result = await svc.RefreshAvailableUpdateAsync();

        Assert.NotNull(result);
        Assert.Equal("2.2.5", result!.Version);
        Assert.Equal("https://x/installer-2.2.5.exe", result.WindowsUrl);
        Assert.Equal("hardened updater", result.ReleaseNotes);
        Assert.NotNull(svc.AvailableUpdate);
        Assert.Equal("2.2.5", svc.AvailableUpdate!.Version);
    }

    [Fact]
    public async Task Refresh_StagesInstaller_InBackground_SoStartInstallIsInstant()
    {
        var installerBytes = new byte[] { 0x4D, 0x5A, 0x90, 0x00, 0x03, 0x00, 0x00, 0x00 };
        var meta = TestHttpHandler.Always(HttpStatusCode.OK,
            """{"version":"2.2.5","windows_url":"https://x/installer-2.2.5.exe"}""");
        var dl = TestHttpHandler.Bytes(installerBytes);
        var svc = BuildService("2.2.4", meta, dl);

        await svc.RefreshAvailableUpdateAsync();

        // Staging is fire-and-forget — poll briefly for UpdateStaged.
        await WaitFor(() => svc.UpdateStaged, TimeSpan.FromSeconds(5));
        Assert.True(svc.UpdateStaged, "expected installer to stage in background");
    }

    [Fact]
    public async Task Refresh_ReturnsNull_WhenMetadataFails()
    {
        var meta = TestHttpHandler.Always(HttpStatusCode.InternalServerError, "boom", "text/plain");
        var dl = TestHttpHandler.Bytes(new byte[] { 0x4D, 0x5A });
        var svc = BuildService("2.2.4", meta, dl);

        var result = await svc.RefreshAvailableUpdateAsync();

        Assert.Null(result);
        Assert.Null(svc.AvailableUpdate);
    }

    // ── Download retry: the regression net for the 2.2.3 hang ───────────────

    [Fact]
    public async Task Download_Retries_AfterTransientFailure_AndEventuallyStages()
    {
        var installerBytes = new byte[] { 0x4D, 0x5A, 0x90, 0x00 };
        var meta = TestHttpHandler.Always(HttpStatusCode.OK,
            """{"version":"2.2.5","windows_url":"https://x/installer-2.2.5.exe"}""");
        // Fail attempt 1 with HttpRequestException, succeed on attempt 2.
        var dl = new TestHttpHandler((req, attempt) =>
        {
            if (attempt == 1) throw new HttpRequestException("simulated transient failure");
            var msg = new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new ByteArrayContent(installerBytes)
            };
            msg.Content.Headers.ContentLength = installerBytes.Length;
            return Task.FromResult(msg);
        });
        var svc = BuildService("2.2.4", meta, dl);

        await svc.RefreshAvailableUpdateAsync();

        // Retry has a 5s backoff between attempts — give it 15s to finish.
        await WaitFor(() => svc.UpdateStaged, TimeSpan.FromSeconds(15));
        Assert.True(svc.UpdateStaged, "expected staging to succeed after one transient failure");
        Assert.True(dl.CallCount >= 2, $"expected at least 2 download attempts, saw {dl.CallCount}");
    }

    [Fact]
    public async Task Download_GivesUp_AfterAllAttemptsFail()
    {
        var meta = TestHttpHandler.Always(HttpStatusCode.OK,
            """{"version":"2.2.5","windows_url":"https://x/installer-2.2.5.exe"}""");
        var dl = new TestHttpHandler((req, attempt) =>
            throw new HttpRequestException("simulated persistent failure"));
        var svc = BuildService("2.2.4", meta, dl);

        await svc.RefreshAvailableUpdateAsync();

        // 3 attempts with 5s + 10s backoff between them — wait 25s to be safe.
        await WaitFor(() => dl.CallCount >= 3, TimeSpan.FromSeconds(25));
        // Brief tail so the third attempt's exception is observed before we assert.
        await Task.Delay(500);

        Assert.False(svc.UpdateStaged, "should not stage when all attempts fail");
        Assert.Equal(3, dl.CallCount);
    }

    // ── StartInstall: staged file must short-circuit the download ───────────
    //
    // Regression: 2.2.6 users saw "Downloading DraftRight" hang for minutes
    // after clicking Yes on the auto-update prompt, even though the silent
    // background staging had already finished. ShowUpdateDialog used to call
    // DownloadAndInstallAsync directly, ignoring UpdateStaged — kicking off a
    // redundant 130MB download with no per-attempt timeout, no retries, no
    // logging. Now ShowUpdateDialog routes through StartInstall, which honors
    // the staged file. This test enforces that.

    [Fact]
    public async Task StartInstall_UsesStagedInstaller_WithoutRedownloading()
    {
        var installerBytes = new byte[] { 0x4D, 0x5A, 0x90, 0x00, 0x03, 0x00, 0x00, 0x00 };
        var meta = TestHttpHandler.Always(HttpStatusCode.OK,
            """{"version":"2.2.6","windows_url":"https://x/installer-2.2.6.exe"}""");
        var dl = TestHttpHandler.Bytes(installerBytes);
        var svc = BuildService("2.2.5", meta, dl);

        string? launchedPath = null;
        string? launchedVersion = null;
        svc.StagedInstallerLauncherForTest = (path, version) =>
        {
            launchedPath = path;
            launchedVersion = version;
        };

        var info = await svc.RefreshAvailableUpdateAsync();
        Assert.NotNull(info);
        await WaitFor(() => svc.UpdateStaged, TimeSpan.FromSeconds(5));
        Assert.True(svc.UpdateStaged, "precondition: stage must complete before testing the install path");

        var downloadCallsAfterStaging = dl.CallCount;

        svc.StartInstall(info!);

        Assert.Equal(downloadCallsAfterStaging, dl.CallCount);
        Assert.NotNull(launchedPath);
        Assert.Equal("2.2.6", launchedVersion);
    }

    // ── NormalizeForPlatform: per-platform pin overrides legacy envelope ────
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

        var n = UpdateService.NormalizeForPlatform(raw, "windows");

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

        var n = UpdateService.NormalizeForPlatform(raw, "windows");

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

        var n = UpdateService.NormalizeForPlatform(raw, "windows");

        Assert.Equal("2.2.5", n.Version);
        Assert.Equal("https://x/installer-2.2.5.exe", n.WindowsUrl);
    }

    // ── End-to-end: phantom-update loop is closed ───────────────────────────

    [Fact]
    public async Task Refresh_ReturnsNull_WhenWindowsPinMatchesCurrent_DespiteHigherTopLevel()
    {
        // The bug report scenario: user on 2.2.10, server top-level says 2.3.1
        // (mac's version), but the Windows row is still 2.2.10. The client
        // must NOT prompt to "install 2.3.1" — there's no real Windows update.
        var meta = TestHttpHandler.Always(HttpStatusCode.OK,
            """{"version":"2.3.1","windows_url":"https://x/installer-2.2.10.exe","platforms":{"windows":{"version":"2.2.10","url":"https://x/installer-2.2.10.exe"},"mac":{"version":"2.3.1","url":"https://x/mac.dmg"}}}""");
        var dl = TestHttpHandler.Bytes(new byte[] { 0x4D, 0x5A });
        var svc = BuildService("2.2.10", meta, dl);

        var result = await svc.RefreshAvailableUpdateAsync();

        Assert.Null(result);
        Assert.Null(svc.AvailableUpdate);
    }

    [Fact]
    public async Task Refresh_FetchesWithPlatformQuery_SoBackendCanAnchorEnvelope()
    {
        // Server-side: a backend that honors `?platform=windows` returns the
        // Windows row's version as the top-level `version`. We don't need to
        // assert the response handling here (other tests cover that) — just
        // that the client tells the backend which platform it is.
        var meta = TestHttpHandler.Always(HttpStatusCode.OK,
            """{"version":"2.2.10","windows_url":"https://x/installer-2.2.10.exe"}""");
        var dl = TestHttpHandler.Bytes(new byte[] { 0x4D, 0x5A });
        var svc = BuildService("2.2.10", meta, dl);

        await svc.RefreshAvailableUpdateAsync();

        Assert.NotEmpty(meta.Requests);
        var url = meta.Requests[0].RequestUri!.ToString();
        Assert.Contains("/updates/latest", url);
        Assert.Contains("platform=windows", url);
    }

    // ── Helpers ─────────────────────────────────────────────────────────────

    private static async Task WaitFor(Func<bool> condition, TimeSpan timeout)
    {
        var deadline = DateTime.UtcNow + timeout;
        while (DateTime.UtcNow < deadline)
        {
            if (condition()) return;
            await Task.Delay(100);
        }
    }
}
