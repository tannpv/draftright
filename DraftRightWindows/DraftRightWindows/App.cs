using System.Drawing;
using Microsoft.UI.Xaml;
using Microsoft.UI.Dispatching;
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
            Width = 420, Height = 480,
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

        return form;
    }
}
