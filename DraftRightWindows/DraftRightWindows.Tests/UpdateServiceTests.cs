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
