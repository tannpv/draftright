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
    private readonly HttpClient _http = new() { Timeout = TimeSpan.FromSeconds(10) };
    private DateTime _lastCheck = DateTime.MinValue;
    private const int CheckIntervalHours = 24;

    public UpdateService(string currentVersion, string backendUrl)
    {
        _currentVersion = currentVersion;
        _backendUrl = backendUrl.StripTrailingSlash();
    }

    public async Task CheckIfNeededAsync()
    {
        if ((DateTime.UtcNow - _lastCheck).TotalHours < CheckIntervalHours) return;
        _lastCheck = DateTime.UtcNow;

        var info = await FetchLatestVersionAsync();
        if (info == null) return;
        if (!IsNewer(info.Version, _currentVersion)) return;
        if (string.IsNullOrEmpty(info.WindowsUrl)) return;

        ShowUpdateDialog(info);
    }

    public async Task CheckNowAsync()
    {
        _lastCheck = DateTime.UtcNow;

        var info = await FetchLatestVersionAsync();
        if (info == null || !IsNewer(info.Version, _currentVersion))
        {
            System.Windows.Forms.MessageBox.Show(
                $"You're running the latest version (v{_currentVersion}).",
                "No Updates Available",
                System.Windows.Forms.MessageBoxButtons.OK,
                System.Windows.Forms.MessageBoxIcon.Information
            );
            return;
        }
        if (string.IsNullOrEmpty(info.WindowsUrl)) return;

        ShowUpdateDialog(info);
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
        var progressForm = new System.Windows.Forms.Form
        {
            Text = "Updating DraftRight",
            Width = 400, Height = 150,
            StartPosition = System.Windows.Forms.FormStartPosition.CenterScreen,
            FormBorderStyle = System.Windows.Forms.FormBorderStyle.FixedDialog,
            MaximizeBox = false, MinimizeBox = false,
            TopMost = true,
            BackColor = System.Drawing.Color.FromArgb(15, 23, 42),
            ForeColor = System.Drawing.Color.FromArgb(226, 232, 240),
        };

        var statusLabel = new System.Windows.Forms.Label
        {
            Text = $"Downloading DraftRight v{version}...",
            Location = new System.Drawing.Point(20, 20),
            Size = new System.Drawing.Size(340, 20),
            Font = new System.Drawing.Font("Segoe UI", 10),
            ForeColor = System.Drawing.Color.FromArgb(226, 232, 240),
        };

        var progressBar = new System.Windows.Forms.ProgressBar
        {
            Location = new System.Drawing.Point(20, 50),
            Size = new System.Drawing.Size(340, 25),
            Minimum = 0, Maximum = 100, Value = 0,
            Style = System.Windows.Forms.ProgressBarStyle.Continuous,
        };

        var percentLabel = new System.Windows.Forms.Label
        {
            Text = "0%",
            Location = new System.Drawing.Point(20, 80),
            Size = new System.Drawing.Size(340, 20),
            TextAlign = System.Drawing.ContentAlignment.MiddleCenter,
            Font = new System.Drawing.Font("Segoe UI", 9),
            ForeColor = System.Drawing.Color.FromArgb(148, 163, 184),
        };

        progressForm.Controls.AddRange(new System.Windows.Forms.Control[] { statusLabel, progressBar, percentLabel });
        progressForm.Show();

        try
        {
            // Derive the temp filename from the URL so we keep the actual
            // file extension (.exe, .msi, .msix). Hardcoding ".msix" was a
            // legacy assumption from when we shipped the MSIX installer.
            // Now we ship a self-contained single-file .exe.
            var urlFileName = Path.GetFileName(new Uri(url).LocalPath);
            if (string.IsNullOrEmpty(urlFileName))
            {
                urlFileName = $"DraftRight-{version}-setup.exe";
            }
            var tempPath = Path.Combine(Path.GetTempPath(), urlFileName);

            using var response = await _http.GetAsync(url, HttpCompletionOption.ResponseHeadersRead);
            response.EnsureSuccessStatusCode();
            var totalBytes = response.Content.Headers.ContentLength ?? -1;

            using (var stream = await response.Content.ReadAsStreamAsync())
            using (var fileStream = new FileStream(tempPath, FileMode.Create, FileAccess.Write))
            {
                var buffer = new byte[8192];
                long downloaded = 0;
                int bytesRead;

                while ((bytesRead = await stream.ReadAsync(buffer)) > 0)
                {
                    await fileStream.WriteAsync(buffer.AsMemory(0, bytesRead));
                    downloaded += bytesRead;
                    if (totalBytes > 0)
                    {
                        var percent = (int)(downloaded * 100 / totalBytes);
                        progressBar.Value = percent;
                        percentLabel.Text = $"{percent}%";
                    }
                    System.Windows.Forms.Application.DoEvents();
                }
            }

            statusLabel.Text = "Installing...";
            progressBar.Style = System.Windows.Forms.ProgressBarStyle.Marquee;
            percentLabel.Text = "";
            System.Windows.Forms.Application.DoEvents();

            progressForm.Close();

            // For .msi and .msix, just shell-execute and let the OS installer
            // handle replacement. For our self-contained .exe distribution,
            // we need a helper script: wait for the current process to exit,
            // copy new exe over current exe, launch new exe.
            var ext = Path.GetExtension(tempPath).ToLowerInvariant();
            if (ext == ".exe")
            {
                LaunchExeReplacer(tempPath);
            }
            else
            {
                // .msi / .msix / anything else: hand off to OS shell.
                Process.Start(new ProcessStartInfo
                {
                    FileName = tempPath,
                    UseShellExecute = true
                });
            }

            Environment.Exit(0);
        }
        catch (Exception ex)
        {
            progressForm.Close();
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
