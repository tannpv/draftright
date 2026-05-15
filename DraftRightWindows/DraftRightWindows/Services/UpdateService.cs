using System.Net.Http;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Diagnostics;
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

    [JsonPropertyName("release_notes")]
    public string ReleaseNotes { get; set; } = "";

    [JsonPropertyName("required")]
    public bool Required { get; set; }
}

public class UpdateService
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
            DRLogger.Log("Update check: /updates/latest returned null (network or parse error)", DRLogger.Category.APP);
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
        if (applicable != null && !UpdateStaged)
        {
            _ = StageInstallerAsync(applicable);
        }
        return applicable;
    }

    /// <summary>
    /// Begins installing the given update. If its installer was already
    /// staged in the background, runs it immediately (no download wait);
    /// otherwise falls back to download-then-install with a progress window.
    /// Public entry point for the Settings link / tray menu affordances.
    /// </summary>
    public void StartInstall(UpdateInfo info)
    {
        if (string.IsNullOrEmpty(info.WindowsUrl))
        {
            DRLogger.Log($"StartInstall {info.Version}: refused — empty windows_url", DRLogger.Category.APP);
            return;
        }
        if (UpdateStaged && _stagedInstallerPath != null && _stagedVersion == info.Version)
        {
            DRLogger.Log($"StartInstall {info.Version}: using staged installer at {_stagedInstallerPath}", DRLogger.Category.APP);
            RunStagedInstaller(_stagedInstallerPath, info.Version);
        }
        else
        {
            DRLogger.Log($"StartInstall {info.Version}: not staged, falling back to download-then-install from {info.WindowsUrl}", DRLogger.Category.APP);
            _ = DownloadAndInstallAsync(info.WindowsUrl, info.Version);
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
            var path = await TryDownloadInstallerAsync(info.WindowsUrl, info.Version);
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
            DRLogger.Log($"Update staging failed: {ex.Message}", DRLogger.Category.APP);
        }
    }

    /// <summary>
    /// Downloads <paramref name="url"/> to a temp file, retrying on failure
    /// (stalled connections, transient errors). Cache-busting + a per-attempt
    /// timeout so a dead socket can't hang forever. Returns the path or null.
    /// </summary>
    private async Task<string?> TryDownloadInstallerAsync(string url, string version, int attempts = 3)
    {
        var name = Path.GetFileName(new Uri(url).LocalPath);
        if (string.IsNullOrEmpty(name)) name = $"DraftRight-{version}-setup.exe";
        var dest = Path.Combine(Path.GetTempPath(), name);

        for (int attempt = 1; attempt <= attempts; attempt++)
        {
            try
            {
                DRLogger.Log($"Update download attempt {attempt}/{attempts}: GET {url} → {dest}", DRLogger.Category.APP);
                using var req = new HttpRequestMessage(HttpMethod.Get, url);
                req.Headers.CacheControl =
                    new System.Net.Http.Headers.CacheControlHeaderValue { NoCache = true };
                using var resp = await _downloadHttp.SendAsync(req, HttpCompletionOption.ResponseHeadersRead);
                resp.EnsureSuccessStatusCode();
                var size = resp.Content.Headers.ContentLength;
                DRLogger.Log($"Update download attempt {attempt}: headers OK (status {(int)resp.StatusCode}, content-length {(size?.ToString() ?? "?")} bytes)", DRLogger.Category.APP);
                using var cts = new System.Threading.CancellationTokenSource(TimeSpan.FromMinutes(10));
                using (var src = await resp.Content.ReadAsStreamAsync())
                using (var dst = new FileStream(dest, FileMode.Create, FileAccess.Write))
                {
                    await src.CopyToAsync(dst, cts.Token);
                }
                var actual = new FileInfo(dest).Length;
                DRLogger.Log($"Update download attempt {attempt}: wrote {actual} bytes to {dest}", DRLogger.Category.APP);
                return dest;
            }
            catch (Exception ex)
            {
                DRLogger.Log($"Update download attempt {attempt}/{attempts} failed: {ex.GetType().Name}: {ex.Message}", DRLogger.Category.APP);
                try { File.Delete(dest); } catch { }
                if (attempt < attempts) await Task.Delay(TimeSpan.FromSeconds(5 * attempt));
            }
        }
        DRLogger.Log($"Update download: all {attempts} attempts failed for {url}", DRLogger.Category.APP);
        return null;
    }

    /// <summary>Runs an installer that's already on disk: brief "Installing…"
    /// indicator, hand off to the installer, exit.</summary>
    private static void RunStagedInstaller(string path, string version)
    {
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

    private async Task<UpdateInfo?> FetchLatestVersionAsync()
    {
        try
        {
            var json = await _http.GetStringAsync($"{_backendUrl}/updates/latest");
            return JsonSerializer.Deserialize<UpdateInfo>(json);
        }
        catch
        {
            return null;
        }
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
            _ = DownloadAndInstallAsync(info.WindowsUrl, info.Version);
        }
    }

    private async Task DownloadAndInstallAsync(string url, string version)
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
        try
        {
            ui = UpdateProgressUI.ShowOnNewThread(version);

            // Derive the temp filename from the URL so we keep the actual
            // file extension (.exe, .msi, .msix). Hardcoding ".msix" was a
            // legacy assumption from when we shipped the MSIX installer.
            var urlFileName = Path.GetFileName(new Uri(url).LocalPath);
            if (string.IsNullOrEmpty(urlFileName))
            {
                urlFileName = $"DraftRight-{version}-setup.exe";
            }
            var tempPath = Path.Combine(Path.GetTempPath(), urlFileName);

            using var response = await _downloadHttp.GetAsync(url, HttpCompletionOption.ResponseHeadersRead);
            response.EnsureSuccessStatusCode();
            var totalBytes = response.Content.Headers.ContentLength ?? -1;

            using (var stream = await response.Content.ReadAsStreamAsync())
            using (var fileStream = new FileStream(tempPath, FileMode.Create, FileAccess.Write))
            {
                var buffer = new byte[8192];
                long downloaded = 0;
                int bytesRead;
                int lastReportedPercent = -1;

                while ((bytesRead = await stream.ReadAsync(buffer)) > 0)
                {
                    await fileStream.WriteAsync(buffer.AsMemory(0, bytesRead));
                    downloaded += bytesRead;
                    if (totalBytes > 0)
                    {
                        var percent = (int)(downloaded * 100 / totalBytes);
                        if (percent != lastReportedPercent)
                        {
                            ui.SetProgress(percent);
                            lastReportedPercent = percent;
                        }
                    }
                }
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
    private readonly System.Threading.ManualResetEventSlim _ready = new(false);

    private UpdateProgressUI(string version)
    {
        _form = new System.Windows.Forms.Form
        {
            Text = "Updating DraftRight",
            Width = 420, Height = 160,
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
        _form.Controls.AddRange(new System.Windows.Forms.Control[] { _statusLabel, _progressBar, _percentLabel });

        // The handle isn't created until the form is shown. We need a handle
        // before any BeginInvoke call from another thread can succeed, so
        // signal _ready from HandleCreated on the form's own thread.
        _form.HandleCreated += (_, _) => _ready.Set();
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

    public void SetIndeterminate(string statusText)
    {
        if (!_form.IsHandleCreated) return;
        try
        {
            _form.BeginInvoke(new Action(() =>
            {
                _statusLabel.Text = statusText;
                _progressBar.Style = System.Windows.Forms.ProgressBarStyle.Marquee;
                _percentLabel.Text = "";
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
