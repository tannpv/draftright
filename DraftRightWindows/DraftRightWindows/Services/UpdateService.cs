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
    // Short-timeout client for the small JSON metadata fetch.
    private readonly HttpClient _http = new() { Timeout = TimeSpan.FromSeconds(10) };
    // Long-timeout client for the multi-hundred-MB installer download.
    // 10 minutes lets us cover slow connections without indefinite hangs.
    private readonly HttpClient _downloadHttp = new() { Timeout = TimeSpan.FromMinutes(10) };
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

    /// <summary>Raised whenever <see cref="AvailableUpdate"/> changes.</summary>
    public event Action? AvailableUpdateChanged;

    public string CurrentVersion => _currentVersion;

    public UpdateService(string currentVersion, string backendUrl)
    {
        _currentVersion = currentVersion;
        _backendUrl = backendUrl.StripTrailingSlash();
    }

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
        var info = await FetchLatestVersionAsync();
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
            try { AvailableUpdateChanged?.Invoke(); } catch { /* listener errors are not our problem */ }
        }
        return applicable;
    }

    /// <summary>
    /// Begins downloading + installing the given update. Public entry point
    /// for the Settings link / tray menu "update available" affordances.
    /// </summary>
    public void StartInstall(UpdateInfo info)
    {
        if (string.IsNullOrEmpty(info.WindowsUrl)) return;
        _ = DownloadAndInstallAsync(info.WindowsUrl, info.Version);
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

            // Three cases:
            //  - Inno Setup installer ("DraftRight-Setup-Windows-x.y.z-arch.exe"):
            //    just run it. Its own [Code] PrepareToInstall hook kills the
            //    running app, then it overwrites the install in place. Running
            //    it through LaunchExeReplacer would clobber DraftRightWindows.exe
            //    with the installer binary and then have it taskkill itself.
            //  - Raw self-contained app exe ("DraftRight-Windows-...exe"):
            //    LaunchExeReplacer — wait for exit, copy over current exe, relaunch.
            //  - .msi / .msix: hand off to the OS shell.
            var ext = Path.GetExtension(tempPath).ToLowerInvariant();
            var name = Path.GetFileNameWithoutExtension(tempPath).ToLowerInvariant();
            var looksLikeInstaller = name.Contains("setup") || name.Contains("install");
            if (ext == ".exe" && !looksLikeInstaller)
            {
                LaunchExeReplacer(tempPath);
            }
            else
            {
                // Installer .exe / .msi / .msix / anything else: hand off to OS shell.
                Process.Start(new ProcessStartInfo
                {
                    FileName = tempPath,
                    UseShellExecute = true
                });
            }

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
