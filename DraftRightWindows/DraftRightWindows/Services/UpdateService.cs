using System.Net.Http;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Diagnostics;
using System.Security.Cryptography;
using DraftRightWindows.Helpers;

namespace DraftRightWindows.Services;

public class UpdateInfo
{
    [JsonPropertyName("version")]
    public string Version { get; set; } = "";

    [JsonPropertyName("mac_url")]
    public string MacUrl { get; set; } = "";

    [JsonPropertyName("windows_url")]
    public string WindowsUrl { get; set; } = "";

    [JsonPropertyName("linux_url")]
    public string LinuxUrl { get; set; } = "";

    [JsonPropertyName("windows_sha256")]
    public string WindowsSha256 { get; set; } = "";

    [JsonPropertyName("release_notes")]
    public string ReleaseNotes { get; set; } = "";

    [JsonPropertyName("required")]
    public bool Required { get; set; }

    /// <summary>
    /// Per-platform expansion added by the backend. The Windows entry is the
    /// authoritative source for what to install on this client — the legacy
    /// top-level <see cref="Version"/> is a cross-platform max and can drift
    /// ahead of <see cref="WindowsUrl"/>'s actual version (root cause of the
    /// "current 2.2.10, install 2.3.1, still 2.2.10" loop). Null on legacy
    /// backends; the client falls back to the top-level fields then.
    /// </summary>
    [JsonPropertyName("platforms")]
    public Dictionary<string, PlatformRelease>? Platforms { get; set; }
}

public class PlatformRelease
{
    [JsonPropertyName("version")]
    public string Version { get; set; } = "";

    [JsonPropertyName("url")]
    public string Url { get; set; } = "";

    [JsonPropertyName("sha256")]
    public string Sha256 { get; set; } = "";

    [JsonPropertyName("notes")]
    public string Notes { get; set; } = "";

    [JsonPropertyName("required")]
    public bool Required { get; set; }
}

public class UpdateService : IUpdateService
{
    private readonly string _currentVersion;
    private readonly string _backendUrl;

    // Both HTTP clients share a SocketsHttpHandler with an explicit
    // ConnectTimeout (15s). Without it, a stalled TLS handshake can sit
    // inside Get/SendAsync for the full request-level Timeout — that's how
    // 2.2.3 ended up with an "Updating DraftRight" window open for 20+ min
    // with zero bytes written and no progress. With ConnectTimeout=15s an
    // unreachable server fails fast, the retry loop kicks in, and the user
    // sees a real error instead of a hang.
    private static HttpClient MakeClient(TimeSpan totalTimeout)
    {
        var handler = new SocketsHttpHandler
        {
            ConnectTimeout = TimeSpan.FromSeconds(15),
            PooledConnectionLifetime = TimeSpan.FromMinutes(10),
        };
        return new HttpClient(handler, disposeHandler: true) { Timeout = totalTimeout };
    }

    // Short-timeout client for the small JSON metadata fetch.
    private readonly HttpClient _http;
    // Long-timeout client for the multi-hundred-MB installer download.
    // 10 minutes covers slow connections; ConnectTimeout above prevents
    // hangs *before* the first byte arrives.
    private readonly HttpClient _downloadHttp;
    private DateTime _lastCheck = DateTime.MinValue;
    private const int CheckIntervalHours = 24;

    /// <summary>
    /// The newest release that is actually applicable to this install
    /// (strictly newer version + a non-empty Windows download URL), or null
    /// if we're up to date / haven't checked yet. Updated by
    /// <see cref="RefreshAvailableUpdateAsync"/>. Read by the Settings
    /// "update available" link and the tray menu.
    /// </summary>
    public UpdateInfo? AvailableUpdate { get; private set; }

    /// <summary>Raised whenever <see cref="AvailableUpdate"/> or the staged
    /// state changes (so UI affordances can re-render).</summary>
    public event Action? AvailableUpdateChanged;

    public string CurrentVersion => _currentVersion;

    // Path to a fully pre-downloaded installer for AvailableUpdate (staged
    // silently in the background), and the version it's for. When this matches
    // AvailableUpdate, "install" is instant — no download wait, no hang.
    private string? _stagedInstallerPath;
    private string? _stagedVersion;
    // Active background-staging Task, or null if none is running. Used to
    // dedupe staging kicks so the foreground "Continue in background" handler
    // and the Refresh path can never race two writers onto the same temp file.
    private Task? _stagingTask;
    private readonly object _stagingLock = new();
    // Sentinel for _stagingPercent: no byte progress known yet (download not
    // started, or a chunked response with no Content-Length).
    private const int StagingPercentUnknown = -1;
    // How often the foreground "install" window samples _stagingPercent.
    private const int StagingProgressPollMs = 250;
    // Live percent (0-100) of the in-flight silent staging download, or
    // StagingPercentUnknown. Lets the foreground "install" path show a real
    // progress bar while it waits on staging, instead of a bare marquee that
    // makes a large download look hung (BUG-45).
    private volatile int _stagingPercent = StagingPercentUnknown;

    /// <summary>True when the available update's installer is already on disk
    /// and ready to run.</summary>
    public bool UpdateStaged =>
        _stagedInstallerPath != null
        && _stagedVersion == AvailableUpdate?.Version
        && File.Exists(_stagedInstallerPath);

    public UpdateService(string currentVersion, string backendUrl)
        : this(currentVersion, backendUrl, MakeClient(TimeSpan.FromSeconds(10)), MakeClient(TimeSpan.FromMinutes(10)))
    { }

    /// <summary>
    /// Test seam: lets unit tests inject pre-built HttpClients wired to a fake
    /// HttpMessageHandler. Production uses the param-less ctor which delegates
    /// here with real <see cref="MakeClient"/>-built clients (SocketsHttpHandler
    /// with ConnectTimeout).
    /// </summary>
    internal UpdateService(string currentVersion, string backendUrl, HttpClient http, HttpClient downloadHttp)
    {
        _currentVersion = currentVersion;
        _backendUrl = backendUrl.StripTrailingSlash();
        _http = http;
        _downloadHttp = downloadHttp;
    }

    /// <summary>Test seam: exposes <see cref="IsNewer"/> for pure-logic tests.</summary>
    internal static bool IsNewerForTest(string remote, string local) => IsNewer(remote, local);

    /// <summary>The platform name the Windows client pins on at /updates/latest.</summary>
    private const string PlatformName = "windows";

    public async Task CheckIfNeededAsync()
    {
        if ((DateTime.UtcNow - _lastCheck).TotalHours < CheckIntervalHours) return;
        _lastCheck = DateTime.UtcNow;

        var info = await RefreshAvailableUpdateAsync();
        if (info != null) ShowUpdateDialog(info);
    }

    public async Task CheckNowAsync()
    {
        _lastCheck = DateTime.UtcNow;

        var info = await RefreshAvailableUpdateAsync();
        if (info == null)
        {
            System.Windows.Forms.MessageBox.Show(
                $"You're running the latest version (v{_currentVersion}).",
                "No Updates Available",
                System.Windows.Forms.MessageBoxButtons.OK,
                System.Windows.Forms.MessageBoxIcon.Information
            );
            return;
        }

        ShowUpdateDialog(info);
    }

    /// <summary>
    /// Fetches /updates/latest, recomputes <see cref="AvailableUpdate"/>,
    /// fires <see cref="AvailableUpdateChanged"/> if it changed, and returns
    /// the applicable update (or null). Safe to call from any thread.
    /// </summary>
    public async Task<UpdateInfo?> RefreshAvailableUpdateAsync()
    {
        DRLogger.Log($"Update check: GET {_backendUrl}/updates/latest (current {_currentVersion})", DRLogger.Category.APP);
        var info = await FetchLatestVersionAsync();
        if (info == null)
        {
            DRLogger.Warn("Update check: /updates/latest returned null (network or parse error)", DRLogger.Category.APP);
        }
        else
        {
            DRLogger.Log($"Update check: server reports {info.Version} (windows_url={(string.IsNullOrEmpty(info.WindowsUrl) ? "(empty)" : "set")})", DRLogger.Category.APP);
        }
        var applicable =
            info != null
            && IsNewer(info.Version, _currentVersion)
            && !string.IsNullOrEmpty(info.WindowsUrl)
                ? info
                : null;

        var changed = applicable?.Version != AvailableUpdate?.Version;
        AvailableUpdate = applicable;
        if (changed)
        {
            // A different (or no) update — last stage is stale.
            _stagedInstallerPath = null;
            _stagedVersion = null;
            try { AvailableUpdateChanged?.Invoke(); } catch { /* listener errors are not our problem */ }
        }
        // Silently pre-download the installer so "install" is instant later.
        if (applicable != null) EnsureStagingInBackground(applicable);
        return applicable;
    }

    /// <summary>
    /// Idempotent kick for the silent background staging of
    /// <paramref name="info"/>. No-op if the installer is already on disk or
    /// if a previous stage is still running — the "Continue in background"
    /// button on the progress UI and the periodic Refresh both call through
    /// here, so two writers can never land on the same temp file.
    /// </summary>
    private void EnsureStagingInBackground(UpdateInfo info)
    {
        lock (_stagingLock)
        {
            if (UpdateStaged) return;
            if (_stagingTask is { IsCompleted: false }) return;
            _stagingTask = StageInstallerAsync(info);
        }
    }

    /// <summary>
    /// Begins installing the given update. If its installer was already
    /// staged in the background, runs it immediately (no download wait);
    /// otherwise falls back to download-then-install with a progress window.
    /// Public entry point for the Settings link / tray menu affordances AND
    /// the "Yes" button on the auto/manual update prompt — every install
    /// pathway must go through here so the staged installer is honored. Doing
    /// a fresh download when one already sits in temp is exactly how 2.2.6
    /// users got stuck on "Downloading DraftRight" again.
    /// </summary>
    public void StartInstall(UpdateInfo info)
    {
        if (string.IsNullOrEmpty(info.WindowsUrl))
        {
            DRLogger.Warn($"StartInstall {info.Version}: refused — empty windows_url", DRLogger.Category.APP);
            return;
        }
        if (UpdateStaged && _stagedInstallerPath != null && _stagedVersion == info.Version)
        {
            DRLogger.Log($"StartInstall {info.Version}: using staged installer at {_stagedInstallerPath}", DRLogger.Category.APP);
            RunStagedInstaller(_stagedInstallerPath, info.Version);
            return;
        }

        // Not staged yet. If silent background staging is already downloading
        // this same version, DO NOT start a second download — both write the
        // same temp file (Path.GetTempPath()/<name>) with FileMode.Create, so
        // the second writer throws "The process cannot access the file ...
        // because it is being used by another process" on every attempt and
        // the foreground path dies with "Update failed: could not download
        // installer after multiple attempts" even though staging is fine.
        // Instead, wait on the in-flight staging behind a progress window and
        // install the staged file when it lands.
        Task? staging;
        lock (_stagingLock) { staging = _stagingTask; }
        if (staging is { IsCompleted: false })
        {
            DRLogger.Log($"StartInstall {info.Version}: staging already in flight — waiting on it instead of starting a second download", DRLogger.Category.APP);
            _ = AwaitStagingThenInstallAsync(info, staging);
        }
        else
        {
            DRLogger.Log($"StartInstall {info.Version}: not staged, falling back to download-then-install from {info.WindowsUrl}", DRLogger.Category.APP);
            _ = DownloadAndInstallAsync(info.WindowsUrl, info.Version, ResolveWindowsSha256(info));
        }
    }

    /// <summary>
    /// Waits on the in-flight silent staging download (rather than racing it
    /// with a second writer to the same temp file), then runs the staged
    /// installer. Shows an indeterminate progress window — silent staging
    /// reports no byte progress, so a marquee is the honest visual — with a
    /// "Continue in background" button that drops the window and lets staging
    /// finish on its own (the tray / Settings "ready to install" affordance
    /// fires via <see cref="AvailableUpdateChanged"/> when it lands).
    /// </summary>
    private async Task AwaitStagingThenInstallAsync(UpdateInfo info, Task staging)
    {
        // Test path: the headless test host has no WinForms desktop, so skip
        // the progress window entirely and hand off through RunStagedInstaller
        // (which honors StagedInstallerLauncherForTest).
        if (StagedInstallerLauncherForTest != null)
        {
            try { await staging; } catch { /* staging logs its own failures */ }
            if (UpdateStaged && _stagedInstallerPath != null && _stagedVersion == info.Version)
                RunStagedInstaller(_stagedInstallerPath, info.Version);
            return;
        }

        UpdateProgressUI? ui = null;
        using var backgroundCts = new System.Threading.CancellationTokenSource();
        using var pollCts = new System.Threading.CancellationTokenSource();
        try
        {
            ui = UpdateProgressUI.ShowOnNewThread(info.Version);
            ui.EnableBackground();
            ui.SetIndeterminate($"Downloading DraftRight v{info.Version}...", hideBackgroundButton: false);
            ui.CancelRequested += backgroundCts.Cancel;

            // Mirror the silent staging download's live percent onto the
            // progress window so a large download shows a moving bar instead of
            // a bare marquee that reads as "hung forever" (BUG-45). Falls back
            // to the marquee while percent is unknown (-1: no Content-Length or
            // not started yet).
            UpdateProgressUI localUi = ui;
            _ = Task.Run(async () =>
            {
                int shownPct = StagingPercentUnknown;
                bool marquee = true;
                while (!pollCts.IsCancellationRequested)
                {
                    int pct = _stagingPercent;
                    if (pct != StagingPercentUnknown)
                    {
                        if (pct != shownPct) { localUi.SetProgress(pct); shownPct = pct; }
                        marquee = false;
                    }
                    else if (!marquee)
                    {
                        localUi.SetIndeterminate($"Downloading DraftRight v{info.Version}...", hideBackgroundButton: false);
                        marquee = true;
                    }
                    try { await Task.Delay(StagingProgressPollMs, pollCts.Token); }
                    catch (OperationCanceledException) { break; }
                }
            });

            // Either staging finishes, or the user clicks "Continue in background".
            var backgrounded = Task.Delay(System.Threading.Timeout.Infinite, backgroundCts.Token);
            await Task.WhenAny(staging, backgrounded);
            pollCts.Cancel();

            if (backgroundCts.IsCancellationRequested)
            {
                DRLogger.Log($"AwaitStagingThenInstall {info.Version}: backgrounded by user — staging continues silently", DRLogger.Category.APP);
                ui.Close();
                return;
            }

            ui.Close();

            if (UpdateStaged && _stagedInstallerPath != null && _stagedVersion == info.Version)
            {
                RunStagedInstaller(_stagedInstallerPath, info.Version);
            }
            else
            {
                // Staging completed without producing a usable file (download
                // failed). Fall back to the retrying download-then-install path.
                DRLogger.Log($"AwaitStagingThenInstall {info.Version}: staging ended without a staged file — downloading directly", DRLogger.Category.APP);
                await DownloadAndInstallAsync(info.WindowsUrl, info.Version, ResolveWindowsSha256(info));
            }
        }
        catch (Exception ex)
        {
            ui?.Close();
            System.Windows.Forms.MessageBox.Show(
                $"Update failed: {ex.Message}",
                "Update Error",
                System.Windows.Forms.MessageBoxButtons.OK,
                System.Windows.Forms.MessageBoxIcon.Error
            );
        }
    }

    /// <summary>
    /// Silently downloads the available update's installer to a temp file in
    /// the background — no UI, retries on stalls/failures. When it lands,
    /// <see cref="UpdateStaged"/> flips true and <see cref="AvailableUpdateChanged"/>
    /// fires so the "update available" affordances can say "ready to install".
    /// </summary>
    private async Task StageInstallerAsync(UpdateInfo info)
    {
        try
        {
            _stagingPercent = StagingPercentUnknown;
            var path = await TryDownloadInstallerAsync(
                info.WindowsUrl, info.Version,
                expectedSha256: ResolveWindowsSha256(info),
                onProgress: pct => _stagingPercent = pct);
            if (path == null) return;
            // Guard against AvailableUpdate having changed while we downloaded.
            if (AvailableUpdate?.Version != info.Version)
            {
                try { File.Delete(path); } catch { }
                return;
            }
            _stagedInstallerPath = path;
            _stagedVersion = info.Version;
            DRLogger.Log($"Update {info.Version} staged at {path}", DRLogger.Category.APP);
            try { AvailableUpdateChanged?.Invoke(); } catch { }
        }
        catch (Exception ex)
        {
            DRLogger.Error($"Update staging failed: {ex.Message}", DRLogger.Category.APP);
        }
    }

    /// <summary>
    /// Downloads <paramref name="url"/> to a temp file, retrying on failure
    /// (stalled connections, transient errors). Cache-busting + a per-attempt
    /// timeout so a dead socket can't hang forever. Returns the path or null.
    ///
    /// When <paramref name="ui"/> is non-null, reports percent progress to it
    /// as bytes land — used by the interactive "Yes, install now" path. When
    /// null, downloads silently for background staging.
    ///
    /// <paramref name="externalCt"/> lets the foreground caller bail (e.g.
    /// the "Continue in background" button on the progress UI). When it
    /// trips, the retry loop exits immediately, the partial file is deleted,
    /// and we return null without logging it as a failure.
    /// </summary>
    /// <summary>
    /// Effective Windows artifact hash from the manifest: the per-platform
    /// entry wins, falling back to the legacy top-level field. Lowercased;
    /// empty when the release predates hash publishing.
    /// </summary>
    internal static string ResolveWindowsSha256(UpdateInfo info)
    {
        var fromPlatform = info.Platforms != null
            && info.Platforms.TryGetValue("windows", out var pin) && pin != null
            ? pin.Sha256 : null;
        var hash = !string.IsNullOrEmpty(fromPlatform) ? fromPlatform : info.WindowsSha256;
        return (hash ?? "").Trim().ToLowerInvariant();
    }

    private static string Sha256OfFile(string path)
    {
        using var sha = SHA256.Create();
        using var fs = File.OpenRead(path);
        return Convert.ToHexString(sha.ComputeHash(fs)).ToLowerInvariant();
    }

    internal async Task<string?> TryDownloadInstallerAsync(string url, string version, UpdateProgressUI? ui = null, int attempts = 3, System.Threading.CancellationToken externalCt = default, string? expectedSha256 = null, Action<int>? onProgress = null)
    {
        var name = Path.GetFileName(new Uri(url).LocalPath);
        if (string.IsNullOrEmpty(name)) name = $"DraftRight-{version}-setup.exe";
        var dest = Path.Combine(Path.GetTempPath(), name);

        for (int attempt = 1; attempt <= attempts; attempt++)
        {
            if (externalCt.IsCancellationRequested)
            {
                DRLogger.Log($"Update download canceled by caller before attempt {attempt}", DRLogger.Category.APP);
                if (_stagedInstallerPath != dest)
                {
                    try { File.Delete(dest); } catch { }
                }
                return null;
            }
            try
            {
                DRLogger.Log($"Update download attempt {attempt}/{attempts}: GET {url} → {dest}", DRLogger.Category.APP);
                using var req = new HttpRequestMessage(HttpMethod.Get, url);
                req.Headers.CacheControl =
                    new System.Net.Http.Headers.CacheControlHeaderValue { NoCache = true };
                // Link the per-attempt 10-minute timeout with the external
                // cancel so either path can break out cleanly.
                using var attemptCts = System.Threading.CancellationTokenSource.CreateLinkedTokenSource(externalCt);
                attemptCts.CancelAfter(TimeSpan.FromMinutes(10));
                using var resp = await _downloadHttp.SendAsync(req, HttpCompletionOption.ResponseHeadersRead, attemptCts.Token);
                resp.EnsureSuccessStatusCode();
                var size = resp.Content.Headers.ContentLength;
                DRLogger.Log($"Update download attempt {attempt}: headers OK (status {(int)resp.StatusCode}, content-length {(size?.ToString() ?? "?")} bytes)", DRLogger.Category.APP);
                using (var src = await resp.Content.ReadAsStreamAsync(attemptCts.Token))
                using (var dst = new FileStream(dest, FileMode.Create, FileAccess.Write))
                {
                    if (size > 0 && (ui != null || onProgress != null))
                    {
                        // Report percent to the foreground UI (interactive
                        // install) AND/OR the onProgress callback (silent
                        // staging feeding _stagingPercent, so the awaited
                        // "install" path can show a real bar — BUG-45).
                        void Report(int pct) { ui?.SetProgress(pct); onProgress?.Invoke(pct); }
                        await CopyWithProgressAsync(src, dst, size.Value, Report, attemptCts.Token);
                    }
                    else
                    {
                        // No Content-Length (chunked transfer / some CDNs): we
                        // can't compute a percentage, so show a moving marquee
                        // instead of a Continuous bar frozen at 0% — otherwise a
                        // download that IS progressing looks hung.
                        ui?.SetIndeterminate($"Downloading DraftRight v{version}...", hideBackgroundButton: false);
                        await src.CopyToAsync(dst, attemptCts.Token);
                    }
                }
                var actual = new FileInfo(dest).Length;
                DRLogger.Log($"Update download attempt {attempt}: wrote {actual} bytes to {dest}", DRLogger.Category.APP);

                // Integrity check: when the manifest published a hash, the
                // downloaded installer must match before we ever execute it.
                // A mismatch means a corrupted or tampered artifact — delete it
                // and fail the attempt (retry), never run it.
                if (!string.IsNullOrEmpty(expectedSha256))
                {
                    var got = Sha256OfFile(dest);
                    if (!string.Equals(got, expectedSha256, StringComparison.OrdinalIgnoreCase))
                    {
                        DRLogger.Error($"Update integrity check FAILED on attempt {attempt}: expected {expectedSha256}, got {got}. Discarding.", DRLogger.Category.APP);
                        try { File.Delete(dest); } catch { }
                        continue;
                    }
                    DRLogger.Log($"Update integrity check passed (sha256 match) for {dest}", DRLogger.Category.APP);
                }
                else
                {
                    DRLogger.Warn($"No sha256 published for {version} — installing unverified.", DRLogger.Category.APP);
                }
                return dest;
            }
            catch (OperationCanceledException) when (externalCt.IsCancellationRequested)
            {
                DRLogger.Log($"Update download canceled by caller during attempt {attempt}", DRLogger.Category.APP);
                // Don't blow away a fresh staged installer if a concurrent
                // silent staging just landed bytes at the same temp path.
                if (_stagedInstallerPath != dest)
                {
                    try { File.Delete(dest); } catch { }
                }
                return null;
            }
            catch (Exception ex)
            {
                DRLogger.Warn($"Update download attempt {attempt}/{attempts} failed: {ex.GetType().Name}: {ex.Message}", DRLogger.Category.APP);
                try { File.Delete(dest); } catch { }
                if (attempt < attempts)
                {
                    try { await Task.Delay(TimeSpan.FromSeconds(5 * attempt), externalCt); }
                    catch (OperationCanceledException) { return null; }
                }
            }
        }
        DRLogger.Error($"Update download: all {attempts} attempts failed for {url}", DRLogger.Category.APP);
        return null;
    }

    private static async Task CopyWithProgressAsync(Stream src, Stream dst, long total, Action<int> report, System.Threading.CancellationToken ct)
    {
        var buffer = new byte[81920];
        long downloaded = 0;
        int lastPercent = -1;
        int bytesRead;
        while ((bytesRead = await src.ReadAsync(buffer, ct)) > 0)
        {
            await dst.WriteAsync(buffer.AsMemory(0, bytesRead), ct);
            downloaded += bytesRead;
            var percent = (int)(downloaded * 100 / total);
            if (percent != lastPercent)
            {
                report(percent);
                lastPercent = percent;
            }
        }
    }

    /// <summary>Test seam: overrides the staged-installer launch (UI +
    /// LaunchInstaller + Environment.Exit) with a recorder so unit tests can
    /// assert that the dialog path skips a redundant download. Null in
    /// production — the real launcher runs.</summary>
    internal Action<string, string>? StagedInstallerLauncherForTest { get; set; }

    /// <summary>Runs an installer that's already on disk: brief "Installing…"
    /// indicator, hand off to the installer, exit.</summary>
    private void RunStagedInstaller(string path, string version)
    {
        if (StagedInstallerLauncherForTest != null)
        {
            StagedInstallerLauncherForTest(path, version);
            return;
        }
        UpdateProgressUI? ui = null;
        try
        {
            ui = UpdateProgressUI.ShowOnNewThread(version);
            ui.SetIndeterminate("Installing...");
            LaunchInstaller(path);
            ui.Close();
            Environment.Exit(0);
        }
        catch (Exception ex)
        {
            ui?.Close();
            System.Windows.Forms.MessageBox.Show(
                $"Update failed: {ex.Message}",
                "Update Error",
                System.Windows.Forms.MessageBoxButtons.OK,
                System.Windows.Forms.MessageBoxIcon.Error
            );
        }
    }

    /// <summary>
    /// Release notes the backend currently advertises for <paramref name="version"/>,
    /// or null if there are none or the latest published version no longer
    /// matches (e.g. an even newer release is already out). Used for the
    /// post-update "What's New" notice — right after updating, the latest
    /// version equals the running version, so its notes are the right ones.
    /// </summary>
    public async Task<string?> GetReleaseNotesForVersionAsync(string version)
    {
        var info = await FetchLatestVersionAsync();
        if (info == null || info.Version != version) return null;
        return string.IsNullOrWhiteSpace(info.ReleaseNotes) ? null : info.ReleaseNotes;
    }

    private async Task<UpdateInfo?> FetchLatestVersionAsync()
    {
        try
        {
            // Pass platform=windows so the backend anchors top-level `version`
            // / `release_notes` / `required` on the Windows row (belt). The
            // client *also* normalizes against `platforms.windows` below
            // (suspenders) so an older backend that ignores the query param
            // still can't trick us into installing the wrong version.
            var json = await _http.GetStringAsync($"{_backendUrl}/updates/latest?platform={PlatformName}");
            var raw = JsonSerializer.Deserialize<UpdateInfo>(json);
            return raw == null ? null : NormalizeForPlatform(raw, PlatformName);
        }
        catch
        {
            return null;
        }
    }

    /// <summary>
    /// Returns a copy of <paramref name="raw"/> whose top-level
    /// <see cref="UpdateInfo.Version"/> / <see cref="UpdateInfo.WindowsUrl"/>
    /// / notes / required are taken from <c>platforms[platform]</c> when that
    /// entry is present and well-formed. That entry is tied to the same DB
    /// row as <c>windows_url</c>, so the two can never drift — unlike the
    /// legacy envelope which used a cross-platform max for `version`. Falls
    /// through unchanged when the backend is too old to emit the
    /// <c>platforms</c> map (or the platform-specific entry is empty), so
    /// older deployments keep working.
    /// </summary>
    internal static UpdateInfo NormalizeForPlatform(UpdateInfo raw, string platform)
    {
        if (raw.Platforms == null) return raw;
        if (!raw.Platforms.TryGetValue(platform, out var pin) || pin == null) return raw;
        if (string.IsNullOrEmpty(pin.Version)) return raw;

        return new UpdateInfo
        {
            Version = pin.Version,
            WindowsUrl = !string.IsNullOrEmpty(pin.Url) ? pin.Url : raw.WindowsUrl,
            WindowsSha256 = !string.IsNullOrEmpty(pin.Sha256) ? pin.Sha256 : raw.WindowsSha256,
            MacUrl = raw.MacUrl,
            LinuxUrl = raw.LinuxUrl,
            ReleaseNotes = !string.IsNullOrEmpty(pin.Notes) ? pin.Notes : raw.ReleaseNotes,
            Required = pin.Required || raw.Required,
            Platforms = raw.Platforms,
        };
    }

    private static bool IsNewer(string remote, string local)
    {
        var r = remote.Split('.').Select(s => int.TryParse(s, out var n) ? n : 0).ToArray();
        var l = local.Split('.').Select(s => int.TryParse(s, out var n) ? n : 0).ToArray();
        var len = Math.Max(r.Length, l.Length);
        for (int i = 0; i < len; i++)
        {
            var rv = i < r.Length ? r[i] : 0;
            var lv = i < l.Length ? l[i] : 0;
            if (rv > lv) return true;
            if (rv < lv) return false;
        }
        return false;
    }

    private void ShowUpdateDialog(UpdateInfo info)
    {
        var message = $"DraftRight v{info.Version} is available.\n\n{info.ReleaseNotes}";
        var buttons = info.Required
            ? System.Windows.Forms.MessageBoxButtons.OK
            : System.Windows.Forms.MessageBoxButtons.YesNo;

        var result = System.Windows.Forms.MessageBox.Show(
            message,
            "Update Available",
            buttons,
            System.Windows.Forms.MessageBoxIcon.Information
        );

        if (result == System.Windows.Forms.DialogResult.Yes ||
            result == System.Windows.Forms.DialogResult.OK)
        {
            StartInstall(info);
        }
    }

    private async Task DownloadAndInstallAsync(string url, string version, string? expectedSha256 = null)
    {
        // The progress form must live on a thread that has a WinForms message
        // pump. The caller of this method (the update check) runs on a
        // thread-pool thread, which has no pump — so creating a non-modal form
        // there and calling Application.DoEvents() does nothing, the form
        // never paints (you get the blank/black "Updating... (Not Responding)"
        // window) and the user thinks the app froze.
        //
        // Fix: spin a dedicated STA thread that runs Application.Run(form).
        // The download itself runs on the original thread (async I/O — no UI
        // needed), and we marshal status / progress updates onto the form's
        // thread via BeginInvoke.

        UpdateProgressUI? ui = null;
        using var backgroundCts = new System.Threading.CancellationTokenSource();
        try
        {
            ui = UpdateProgressUI.ShowOnNewThread(version);
            // "Continue in background" is only meaningful while we own the
            // download. The button is hidden by default and Surface'd here.
            ui.EnableBackground();
            ui.CancelRequested += backgroundCts.Cancel;

            // Reuse the hardened staging download path: retries with backoff,
            // per-attempt CTS, structured logging. The dialog-path used to have
            // its own untimed/unretried copy of this loop — that's how 2.2.6
            // users got the second "Downloading DraftRight" hang even after
            // 2.2.5 hardened staging.
            var tempPath = await TryDownloadInstallerAsync(url, version, ui, externalCt: backgroundCts.Token, expectedSha256: expectedSha256);

            if (backgroundCts.IsCancellationRequested)
            {
                // User chose "Continue in background". Drop the foreground UI
                // and make sure the silent staging path is running so the
                // tray/Settings "Update available" affordance can flip to
                // "ready to install" as soon as the bytes land.
                DRLogger.Log($"DownloadAndInstall {version}: backgrounded by user — handing off to silent staging", DRLogger.Category.APP);
                ui.Close();
                if (AvailableUpdate != null) EnsureStagingInBackground(AvailableUpdate);
                return;
            }

            if (tempPath == null)
            {
                ui.Close();
                System.Windows.Forms.MessageBox.Show(
                    "Update failed: could not download installer after multiple attempts. Check your connection and try again from Settings.",
                    "Update Error",
                    System.Windows.Forms.MessageBoxButtons.OK,
                    System.Windows.Forms.MessageBoxIcon.Error
                );
                return;
            }

            ui.SetIndeterminate("Installing...");
            LaunchInstaller(tempPath);
            ui.Close();
            Environment.Exit(0);
        }
        catch (Exception ex)
        {
            ui?.Close();
            System.Windows.Forms.MessageBox.Show(
                $"Update failed: {ex.Message}",
                "Update Error",
                System.Windows.Forms.MessageBoxButtons.OK,
                System.Windows.Forms.MessageBoxIcon.Error
            );
        }
    }

    /// <summary>
    /// Hands a downloaded artifact off for installation. Three cases:
    ///  - Inno Setup installer ("DraftRight-Setup-Windows-x.y.z-arch.exe"):
    ///    just run it — its own [Code] PrepareToInstall hook kills the running
    ///    app, then it overwrites the install in place. Running it through
    ///    LaunchExeReplacer would clobber DraftRightWindows.exe with the
    ///    installer binary and then have it taskkill itself.
    ///  - Raw self-contained app exe ("DraftRight-Windows-...exe"):
    ///    LaunchExeReplacer — wait for exit, copy over current exe, relaunch.
    ///  - .msi / .msix / anything else: hand off to the OS shell.
    /// </summary>
    private static void LaunchInstaller(string path)
    {
        var ext = Path.GetExtension(path).ToLowerInvariant();
        var name = Path.GetFileNameWithoutExtension(path).ToLowerInvariant();
        var looksLikeInstaller = name.Contains("setup") || name.Contains("install");
        if (ext == ".exe" && !looksLikeInstaller)
        {
            LaunchExeReplacer(path);
        }
        else
        {
            Process.Start(new ProcessStartInfo { FileName = path, UseShellExecute = true });
        }
    }

    /// <summary>
    /// Spawns a hidden PowerShell helper that waits for this process to
    /// exit, then atomically replaces the current EXE with the downloaded
    /// new one and launches the new EXE. The helper outlives this process
    /// so the running .exe can finish releasing its file lock before we
    /// try to overwrite it.
    /// </summary>
    private static void LaunchExeReplacer(string newExePath)
    {
        var currentExe = Environment.ProcessPath
            ?? Process.GetCurrentProcess().MainModule?.FileName
            ?? throw new InvalidOperationException("Cannot determine current EXE path");

        var pid = Environment.ProcessId;

        // PowerShell script: wait for old process to exit (5s grace),
        // replace exe, relaunch, clean up temp.
        var ps = string.Join(";", new[]
        {
            $"$ErrorActionPreference = 'SilentlyContinue'",
            $"try {{ Wait-Process -Id {pid} -Timeout 30 }} catch {{ }}",
            $"Start-Sleep -Milliseconds 500",  // extra safety for file-lock release
            $"Copy-Item -LiteralPath '{newExePath.Replace("'", "''")}' -Destination '{currentExe.Replace("'", "''")}' -Force",
            $"Start-Process -FilePath '{currentExe.Replace("'", "''")}'",
            $"Remove-Item -LiteralPath '{newExePath.Replace("'", "''")}' -Force"
        });

        Process.Start(new ProcessStartInfo
        {
            FileName = "powershell.exe",
            Arguments = $"-NoProfile -WindowStyle Hidden -Command \"{ps.Replace("\"", "\\\"")}\"",
            UseShellExecute = false,
            CreateNoWindow = true
        });
    }
}

/// <summary>
/// The update progress window. Runs on its own STA thread with a WinForms
/// message pump (Application.Run) so it paints reliably even though the
/// caller (update check) lives on a thread-pool thread.
///
/// Cross-thread API: ShowOnNewThread starts the pump; SetProgress /
/// SetIndeterminate / Close marshal back to the form's thread via
/// BeginInvoke and are safe to call from anywhere.
/// </summary>
internal sealed class UpdateProgressUI
{
    private readonly System.Windows.Forms.Form _form;
    private readonly System.Windows.Forms.Label _statusLabel;
    private readonly System.Windows.Forms.ProgressBar _progressBar;
    private readonly System.Windows.Forms.Label _percentLabel;
    private readonly System.Windows.Forms.Button _backgroundButton;
    private readonly System.Threading.ManualResetEventSlim _ready = new(false);

    /// <summary>
    /// Fires when the user clicks "Continue in background". The caller is
    /// expected to cancel the in-flight foreground download and ensure the
    /// silent staging path picks up where this one left off — see
    /// <see cref="UpdateService.EnsureStagingInBackground"/>.
    /// </summary>
    public event Action? CancelRequested;

    private UpdateProgressUI(string version)
    {
        _form = new System.Windows.Forms.Form
        {
            Text = "Updating DraftRight",
            Width = 420, Height = 200,
            StartPosition = System.Windows.Forms.FormStartPosition.CenterScreen,
            FormBorderStyle = System.Windows.Forms.FormBorderStyle.FixedDialog,
            MaximizeBox = false, MinimizeBox = false, ControlBox = false,
            TopMost = true,
            BackColor = System.Drawing.Color.FromArgb(15, 23, 42),
            ForeColor = System.Drawing.Color.FromArgb(226, 232, 240),
        };

        _statusLabel = new System.Windows.Forms.Label
        {
            Text = $"Downloading DraftRight v{version}...",
            Location = new System.Drawing.Point(20, 20),
            Size = new System.Drawing.Size(360, 22),
            Font = new System.Drawing.Font("Segoe UI", 10),
            ForeColor = System.Drawing.Color.FromArgb(226, 232, 240),
        };
        _progressBar = new System.Windows.Forms.ProgressBar
        {
            Location = new System.Drawing.Point(20, 50),
            Size = new System.Drawing.Size(360, 25),
            Minimum = 0, Maximum = 100, Value = 0,
            Style = System.Windows.Forms.ProgressBarStyle.Continuous,
        };
        _percentLabel = new System.Windows.Forms.Label
        {
            Text = "0%",
            Location = new System.Drawing.Point(20, 82),
            Size = new System.Drawing.Size(360, 20),
            TextAlign = System.Drawing.ContentAlignment.MiddleCenter,
            Font = new System.Drawing.Font("Segoe UI", 9),
            ForeColor = System.Drawing.Color.FromArgb(148, 163, 184),
        };
        // Hidden by default — the caller flips it on with EnableBackground()
        // only on the download path. The staged-install path (RunStagedInstaller)
        // would show this for a fraction of a second before flipping to
        // "Installing..." anyway, where the button is meaningless.
        _backgroundButton = new System.Windows.Forms.Button
        {
            Text = "Continue in background",
            Location = new System.Drawing.Point(210, 115),
            Size = new System.Drawing.Size(180, 32),
            FlatStyle = System.Windows.Forms.FlatStyle.Flat,
            BackColor = System.Drawing.Color.FromArgb(30, 41, 59),
            ForeColor = System.Drawing.Color.FromArgb(226, 232, 240),
            Font = new System.Drawing.Font("Segoe UI", 9),
            UseVisualStyleBackColor = false,
            Visible = false,
            Cursor = System.Windows.Forms.Cursors.Hand,
        };
        _backgroundButton.FlatAppearance.BorderColor = System.Drawing.Color.FromArgb(71, 85, 105);
        _backgroundButton.Click += (_, _) =>
        {
            // Disable + reflect the new state immediately so a double-click
            // can't fire CancelRequested twice and so the user gets visual
            // confirmation while we tear down the foreground download.
            _backgroundButton.Enabled = false;
            _statusLabel.Text = "Continuing in background...";
            try { CancelRequested?.Invoke(); } catch { /* listener errors are not our problem */ }
        };
        var tip = new System.Windows.Forms.ToolTip();
        tip.SetToolTip(_backgroundButton, "Hide this window. The download keeps running silently — you'll get a 'ready to install' notice when it's done.");

        _form.Controls.AddRange(new System.Windows.Forms.Control[] {
            _statusLabel, _progressBar, _percentLabel, _backgroundButton,
        });

        // The handle isn't created until the form is shown. We need a handle
        // before any BeginInvoke call from another thread can succeed, so
        // signal _ready from HandleCreated on the form's own thread.
        _form.HandleCreated += (_, _) => _ready.Set();
    }

    /// <summary>
    /// Reveals the "Continue in background" button. Call this on the
    /// download-then-install path after the UI is up — staged installs skip
    /// it because the install step that follows isn't cancelable.
    /// </summary>
    public void EnableBackground()
    {
        if (!_form.IsHandleCreated) return;
        try
        {
            _form.BeginInvoke(new Action(() =>
            {
                _backgroundButton.Visible = true;
                _backgroundButton.Enabled = true;
            }));
        }
        catch (InvalidOperationException) { /* form closing */ }
    }

    public static UpdateProgressUI ShowOnNewThread(string version)
    {
        var ui = new UpdateProgressUI(version);
        var thread = new System.Threading.Thread(() =>
        {
            System.Windows.Forms.Application.Run(ui._form);
        });
        thread.SetApartmentState(System.Threading.ApartmentState.STA);
        thread.IsBackground = true;
        thread.Start();
        // Block until the form's handle is created — callers can BeginInvoke immediately after.
        ui._ready.Wait(TimeSpan.FromSeconds(5));
        return ui;
    }

    public void SetProgress(int percent)
    {
        if (!_form.IsHandleCreated) return;
        try
        {
            _form.BeginInvoke(new Action(() =>
            {
                _progressBar.Style = System.Windows.Forms.ProgressBarStyle.Continuous;
                _progressBar.Value = Math.Max(0, Math.Min(100, percent));
                _percentLabel.Text = $"{percent}%";
            }));
        }
        catch (InvalidOperationException) { /* form closing */ }
    }

    /// <param name="hideBackgroundButton">True (default) for the post-download
    /// "Installing..." phase, where backgrounding is meaningless because Inno
    /// Setup is about to take over. False while still downloading with unknown
    /// progress (e.g. waiting on silent staging), where "Continue in
    /// background" is still a valid choice.</param>
    public void SetIndeterminate(string statusText, bool hideBackgroundButton = true)
    {
        if (!_form.IsHandleCreated) return;
        try
        {
            _form.BeginInvoke(new Action(() =>
            {
                _statusLabel.Text = statusText;
                _progressBar.Style = System.Windows.Forms.ProgressBarStyle.Marquee;
                _percentLabel.Text = "";
                if (hideBackgroundButton) _backgroundButton.Visible = false;
            }));
        }
        catch (InvalidOperationException) { /* form closing */ }
    }

    public void Close()
    {
        if (!_form.IsHandleCreated) return;
        try { _form.BeginInvoke(new Action(() => _form.Close())); }
        catch (InvalidOperationException) { /* already closing */ }
    }
}
