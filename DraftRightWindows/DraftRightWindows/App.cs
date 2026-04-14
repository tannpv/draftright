using System.Collections.Generic;
using System.Drawing;
using Microsoft.UI.Xaml;
using Microsoft.UI.Dispatching;
using DraftRightWindows.Models;
using DraftRightWindows.Services;
using DraftRightWindows.Views;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows;

public class App : Application
{
    public static SettingsService Settings { get; private set; } = null!;
    public static AuthService Auth { get; private set; } = null!;
    public static ApiClient Api { get; private set; } = null!;
    public static HotkeyService Hotkey { get; private set; } = null!;
    public static ClipboardService Clipboard { get; private set; } = null!;

    private Window? _hiddenWindow;
    private WinForms.Form? _settingsForm;
    private Thread? _trayThread;
    private WinForms.NotifyIcon? _trayIcon;
    private System.Threading.Timer? _healthTimer;
    private WinForms.ToolStripMenuItem? _statusMenuItem;
    public static BackendStatus CurrentStatus { get; private set; } = BackendStatus.Offline;
    private DateTime _lastAutoRecovery = DateTime.MinValue;
    public static UpdateService? UpdateService { get; private set; }

    public App()
    {
        UnhandledException += (_, e) =>
        {
            System.IO.File.AppendAllText(
                System.IO.Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.Desktop), "draftright-crash.log"),
                $"[{DateTime.Now}] {e.Exception}\n");
            e.Handled = true;
        };
    }

    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
        _hiddenWindow = new Window { Title = "DraftRight" };
        var hwnd = WinRT.Interop.WindowNative.GetWindowHandle(_hiddenWindow);
        Helpers.Win32Interop.ShowWindow(hwnd, Helpers.Win32Interop.SW_HIDE);

        Settings = new SettingsService();
        Settings.Load();
        Api = new ApiClient(Settings.BackendUrl);
        Auth = new AuthService();
        Clipboard = new ClipboardService();
        Hotkey = new HotkeyService();

        // Restore saved session or auto-login for testing
        if (Auth.RestoreSession())
        {
            Api.SetToken(Auth.AccessToken!);
        }
        else
        {
            _ = AutoLoginAsync();
        }

        _trayThread = new Thread(RunTrayIcon);
        _trayThread.SetApartmentState(ApartmentState.STA);
        _trayThread.IsBackground = true;
        _trayThread.Start();

        // Start health check — immediate first check, then every 30 seconds
        _healthTimer = new System.Threading.Timer(async _ => await PerformHealthCheckAsync(), null, TimeSpan.Zero, TimeSpan.FromSeconds(30));

        // Start update check — 10 seconds after launch
        UpdateService = new UpdateService("1.0.0", Settings.BackendUrl);
        _ = Task.Delay(TimeSpan.FromSeconds(10)).ContinueWith(_ => UpdateService.CheckIfNeededAsync());
    }

    private void RunTrayIcon()
    {
        WinForms.Application.EnableVisualStyles();

        _trayIcon = new WinForms.NotifyIcon();
        _trayIcon.Text = "DraftRight";

        var exePath = Environment.ProcessPath;
        if (exePath != null)
        {
            var icoPath = System.IO.Path.Combine(System.IO.Path.GetDirectoryName(exePath)!, "Assets", "DraftRight.ico");
            if (System.IO.File.Exists(icoPath))
                _trayIcon.Icon = new Icon(icoPath);
            else
                _trayIcon.Icon = SystemIcons.Application;
        }
        else
        {
            _trayIcon.Icon = SystemIcons.Application;
        }

        var menu = new WinForms.ContextMenuStrip();
        _statusMenuItem = new WinForms.ToolStripMenuItem("Offline") { Enabled = false };
        menu.Items.Add(_statusMenuItem);
        menu.Items.Add(new WinForms.ToolStripSeparator());
        menu.Items.Add("Settings", null, (_, _) => OpenSettings());
        menu.Items.Add(new WinForms.ToolStripSeparator());
        menu.Items.Add("Sign Out", null, (_, _) => { Auth.ClearTokens(); Api.ClearToken(); });
        menu.Items.Add("Quit", null, (_, _) => DoQuit());

        _trayIcon.ContextMenuStrip = menu;
        _trayIcon.DoubleClick += (_, _) => OpenSettings();
        _trayIcon.Visible = true;

        WinForms.Application.Run();
    }

    private async Task PerformHealthCheckAsync()
    {
        var status = await Api.CheckHealthAsync();
        CurrentStatus = status;

        // Update tray icon tooltip and status menu item on the tray thread
        if (_trayIcon != null)
        {
            var label = status switch
            {
                BackendStatus.Connected => "Connected",
                BackendStatus.NotLoggedIn => "Not Logged In",
                BackendStatus.WrongServer => "Wrong Server",
                _ => "Offline"
            };

            try
            {
                _trayIcon.Text = $"DraftRight - {label}";
                if (_statusMenuItem != null)
                    _statusMenuItem.Text = label;
            }
            catch
            {
                // Tray icon may be disposed during shutdown
            }
        }

        // Check for updates (throttled internally to once per 24h)
        if (UpdateService != null)
            await UpdateService.CheckIfNeededAsync();

        // Auto-recovery: if offline and targeting localhost, try to start the backend
        if (status == BackendStatus.Offline && (Settings.BackendUrl?.Contains("localhost") ?? false))
        {
            AttemptAutoRecovery();
        }
    }

    /// <summary>
    /// Run start-server.ps1 to bring up Docker services when backend is offline.
    /// Throttled to at most once every 2 minutes.
    /// </summary>
    private void AttemptAutoRecovery()
    {
        var now = DateTime.UtcNow;
        if ((now - _lastAutoRecovery).TotalSeconds < 120) return;
        _lastAutoRecovery = now;

        // Look for start-server.ps1 next to the exe
        var exePath = Environment.ProcessPath;
        if (exePath == null) return;

        var scriptPath = System.IO.Path.Combine(System.IO.Path.GetDirectoryName(exePath)!, "start-server.ps1");
        if (!System.IO.File.Exists(scriptPath)) return;

        try
        {
            var psi = new System.Diagnostics.ProcessStartInfo
            {
                FileName = "powershell.exe",
                Arguments = $"-ExecutionPolicy Bypass -WindowStyle Hidden -File \"{scriptPath}\"",
                CreateNoWindow = true,
                UseShellExecute = false,
            };
            System.Diagnostics.Process.Start(psi);
        }
        catch
        {
            // Silently fail — next health check will retry
        }
    }

    private async Task AutoLoginAsync()
    {
        try
        {
            var result = await Api.LoginAsync("test@test.com", "password123");
            if (!string.IsNullOrEmpty(result.AccessToken))
            {
                Auth.SaveTokens(result.AccessToken, result.RefreshToken, result.User?.Email);
                Api.SetToken(result.AccessToken);
            }
        }
        catch
        {
            // Silently fail — user can login manually via Settings
        }
    }

    private void OpenSettings()
    {
        if (_settingsForm != null && !_settingsForm.IsDisposed)
        {
            _settingsForm.BringToFront();
            _settingsForm.Activate();
            return;
        }

        _settingsForm = SettingsFormBuilder.Create();
        _settingsForm.Show();
    }

    private void DoQuit()
    {
        _healthTimer?.Dispose();
        Hotkey.Unregister();
        if (_trayIcon != null)
        {
            _trayIcon.Visible = false;
            _trayIcon.Dispose();
            _trayIcon = null;
        }
        _settingsForm?.Close();
        WinForms.Application.ExitThread();
        Environment.Exit(0);
    }
}

// ── Simple WinForms Settings Form builder ──
internal static class SettingsFormBuilder
{
    public static WinForms.Form Create()
    {
        var form = new WinForms.Form
        {
            Text = "DraftRight Settings",
            Width = 420, Height = 560,
            StartPosition = WinForms.FormStartPosition.CenterScreen,
            BackColor = System.Drawing.Color.FromArgb(15, 23, 42),
            ForeColor = System.Drawing.Color.FromArgb(226, 232, 240),
            FormBorderStyle = WinForms.FormBorderStyle.FixedSingle,
            MaximizeBox = false,
        };

        var exePath = Environment.ProcessPath;
        if (exePath != null)
        {
            var icoPath = System.IO.Path.Combine(System.IO.Path.GetDirectoryName(exePath)!, "Assets", "DraftRight.ico");
            if (System.IO.File.Exists(icoPath))
                form.Icon = new Icon(icoPath);
        }

        int y = 20;
        var brandBlue = System.Drawing.Color.FromArgb(93, 135, 255);
        var cardBg = System.Drawing.Color.FromArgb(30, 41, 59);
        var textMuted = System.Drawing.Color.FromArgb(148, 163, 184);
        var textPrimary = System.Drawing.Color.FromArgb(226, 232, 240);

        // Title
        form.Controls.Add(new WinForms.Label
        {
            Text = "DraftRight Settings",
            Font = new System.Drawing.Font("Segoe UI", 18, System.Drawing.FontStyle.Bold),
            ForeColor = textPrimary,
            Location = new System.Drawing.Point(24, y),
            AutoSize = true,
        });
        y += 44;

        // Email
        form.Controls.Add(new WinForms.Label
        {
            Text = "Email", ForeColor = textMuted,
            Font = new System.Drawing.Font("Segoe UI", 9),
            Location = new System.Drawing.Point(24, y), AutoSize = true,
        });
        y += 20;
        var emailBox = new WinForms.TextBox
        {
            Location = new System.Drawing.Point(24, y), Size = new System.Drawing.Size(355, 30),
            BackColor = cardBg, ForeColor = textPrimary,
            BorderStyle = WinForms.BorderStyle.FixedSingle,
            Font = new System.Drawing.Font("Segoe UI", 10),
        };
        form.Controls.Add(emailBox);
        y += 38;

        // Password
        form.Controls.Add(new WinForms.Label
        {
            Text = "Password", ForeColor = textMuted,
            Font = new System.Drawing.Font("Segoe UI", 9),
            Location = new System.Drawing.Point(24, y), AutoSize = true,
        });
        y += 20;
        var passBox = new WinForms.TextBox
        {
            Location = new System.Drawing.Point(24, y), Size = new System.Drawing.Size(355, 30),
            BackColor = cardBg, ForeColor = textPrimary,
            BorderStyle = WinForms.BorderStyle.FixedSingle,
            Font = new System.Drawing.Font("Segoe UI", 10),
            UseSystemPasswordChar = true,
        };
        form.Controls.Add(passBox);
        y += 38;

        // Backend URL
        form.Controls.Add(new WinForms.Label
        {
            Text = "Backend URL", ForeColor = textMuted,
            Font = new System.Drawing.Font("Segoe UI", 9),
            Location = new System.Drawing.Point(24, y), AutoSize = true,
        });
        y += 20;
        var urlBox = new WinForms.TextBox
        {
            Text = App.Settings.BackendUrl ?? "http://localhost:3000",
            Location = new System.Drawing.Point(24, y), Size = new System.Drawing.Size(355, 30),
            BackColor = cardBg, ForeColor = textPrimary,
            BorderStyle = WinForms.BorderStyle.FixedSingle,
            Font = new System.Drawing.Font("Segoe UI", 10),
        };
        form.Controls.Add(urlBox);
        y += 44;

        // Status label
        var statusLabel = new WinForms.Label
        {
            ForeColor = System.Drawing.Color.FromArgb(239, 68, 68),
            Font = new System.Drawing.Font("Segoe UI", 9),
            Location = new System.Drawing.Point(24, y), Size = new System.Drawing.Size(355, 20),
        };
        form.Controls.Add(statusLabel);
        y += 28;

        // Sign In button
        var signInBtn = new WinForms.Button
        {
            Text = "Sign In",
            Location = new System.Drawing.Point(24, y), Size = new System.Drawing.Size(355, 40),
            BackColor = brandBlue, ForeColor = System.Drawing.Color.White,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new System.Drawing.Font("Segoe UI", 11, System.Drawing.FontStyle.Bold),
        };
        signInBtn.FlatAppearance.BorderSize = 0;
        signInBtn.Click += async (_, _) =>
        {
            statusLabel.Text = "";
            signInBtn.Enabled = false;
            try
            {
                App.Api.SetBaseUrl(urlBox.Text.Trim());
                var result = await App.Api.LoginAsync(emailBox.Text.Trim(), passBox.Text);
                if (!string.IsNullOrEmpty(result.AccessToken))
                {
                    App.Auth.SaveTokens(result.AccessToken, result.RefreshToken);
                    App.Api.SetToken(result.AccessToken);
                    App.Settings.BackendUrl = urlBox.Text.Trim();
                    App.Settings.Save();
                    statusLabel.ForeColor = System.Drawing.Color.FromArgb(34, 197, 94);
                    statusLabel.Text = "Signed in successfully!";
                }
                else
                {
                    statusLabel.ForeColor = System.Drawing.Color.FromArgb(239, 68, 68);
                    statusLabel.Text = "Login failed";
                }
            }
            catch (Exception ex)
            {
                statusLabel.ForeColor = System.Drawing.Color.FromArgb(239, 68, 68);
                statusLabel.Text = ex.Message;
            }
            finally
            {
                signInBtn.Enabled = true;
            }
        };
        form.Controls.Add(signInBtn);

        y += 50;

        // Version label
        form.Controls.Add(new WinForms.Label
        {
            Text = $"Version: 1.0.0",
            ForeColor = textMuted,
            Font = new System.Drawing.Font("Segoe UI", 9),
            Location = new System.Drawing.Point(24, y),
            AutoSize = true,
        });
        y += 24;

        // Check for Updates button
        var updateBtn = new WinForms.Button
        {
            Text = "Check for Updates",
            Location = new System.Drawing.Point(24, y),
            Size = new System.Drawing.Size(170, 34),
            BackColor = cardBg,
            ForeColor = textPrimary,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new System.Drawing.Font("Segoe UI", 10),
        };
        updateBtn.FlatAppearance.BorderColor = System.Drawing.Color.FromArgb(51, 65, 85);
        updateBtn.Click += async (_, _) =>
        {
            updateBtn.Enabled = false;
            updateBtn.Text = "Checking...";
            if (App.UpdateService != null)
                await App.UpdateService.CheckNowAsync();
            updateBtn.Text = "Check for Updates";
            updateBtn.Enabled = true;
        };
        form.Controls.Add(updateBtn);

        y += 50;

        // ── Panel Tones section ──────────────────────────────────
        form.Controls.Add(new WinForms.Label
        {
            Text = "Panel Tones",
            Font = new System.Drawing.Font("Segoe UI", 12, System.Drawing.FontStyle.Bold),
            ForeColor = textPrimary,
            Location = new System.Drawing.Point(24, y),
            AutoSize = true,
        });
        y += 28;

        var toneCheckboxes = new Dictionary<Tone, WinForms.CheckBox>();
        foreach (var tone in Enum.GetValues<Tone>())
        {
            var cb = new WinForms.CheckBox
            {
                Text = $"{tone.Icon()}  {tone.DisplayName()}",
                Checked = App.Settings.EnabledTones.Contains(tone.ApiValue()),
                ForeColor = textPrimary,
                BackColor = form.BackColor,
                Font = new System.Drawing.Font("Segoe UI", 10),
                Location = new System.Drawing.Point(24, y),
                AutoSize = true,
            };
            cb.CheckedChanged += (_, _) =>
            {
                var apiVal = tone.ApiValue();
                if (cb.Checked)
                {
                    if (!App.Settings.EnabledTones.Contains(apiVal))
                        App.Settings.EnabledTones.Add(apiVal);
                }
                else
                {
                    App.Settings.EnabledTones.Remove(apiVal);
                }
                App.Settings.Save();
            };
            toneCheckboxes[tone] = cb;
            form.Controls.Add(cb);
            y += 26;
        }

        y += 10;

        // Default Tone dropdown
        form.Controls.Add(new WinForms.Label
        {
            Text = "Default Tone (auto-run)",
            ForeColor = textMuted,
            Font = new System.Drawing.Font("Segoe UI", 9),
            Location = new System.Drawing.Point(24, y),
            AutoSize = true,
        });
        y += 20;

        var defaultToneCombo = new WinForms.ComboBox
        {
            Location = new System.Drawing.Point(24, y),
            Size = new System.Drawing.Size(355, 30),
            BackColor = cardBg,
            ForeColor = textPrimary,
            Font = new System.Drawing.Font("Segoe UI", 10),
            DropDownStyle = WinForms.ComboBoxStyle.DropDownList,
            FlatStyle = WinForms.FlatStyle.Flat,
        };
        defaultToneCombo.Items.Add("(None)");
        int selectedIndex = 0;
        foreach (var tone in Enum.GetValues<Tone>())
        {
            defaultToneCombo.Items.Add(tone.DisplayName());
            if (tone.ApiValue() == App.Settings.DefaultTone)
                selectedIndex = defaultToneCombo.Items.Count - 1;
        }
        defaultToneCombo.SelectedIndex = selectedIndex;
        defaultToneCombo.SelectedIndexChanged += (_, _) =>
        {
            if (defaultToneCombo.SelectedIndex == 0)
            {
                App.Settings.DefaultTone = "";
            }
            else
            {
                var allTones = Enum.GetValues<Tone>();
                App.Settings.DefaultTone = allTones[defaultToneCombo.SelectedIndex - 1].ApiValue();
            }
            App.Settings.Save();
        };
        form.Controls.Add(defaultToneCombo);

        y += 44;

        // Adjust form height to fit all controls
        form.Height = y + 40;

        return form;
    }
}
