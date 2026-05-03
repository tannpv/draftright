using System.Collections.Generic;
using System.Drawing;
using Microsoft.UI.Xaml;
using Microsoft.UI.Dispatching;
using DraftRightWindows.Helpers;
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
    public static TextInjector Injector { get; private set; } = null!;

    private Window? _hiddenWindow;
    private IntPtr _hwnd = IntPtr.Zero;
    private WinForms.Form? _settingsForm;
    private Thread? _trayThread;
    private WinForms.NotifyIcon? _trayIcon;
    private System.Threading.Timer? _healthTimer;
    private WinForms.ToolStripMenuItem? _statusMenuItem;
    public static BackendStatus CurrentStatus { get; private set; } = BackendStatus.Offline;
    private DateTime _lastAutoRecovery = DateTime.MinValue;
    public static UpdateService? UpdateService { get; private set; }

    // ── Rewrite flow ────────────────────────────────────────
    private DispatcherQueue? _dispatcherQueue;
    private RewritePanel? _rewritePanel;
    private IntPtr _sourceWindow = IntPtr.Zero;
    private LoadingIndicator? _loadingIndicator;

    // Must be stored as a field — delegate must stay alive for the lifetime of the subclass
    private Win32Interop.SUBCLASSPROC? _subclassProc;

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
        // Capture the UI-thread dispatcher queue before any async work
        _dispatcherQueue = DispatcherQueue.GetForCurrentThread();

        _hiddenWindow = new Window { Title = "DraftRight" };
        _hwnd = WinRT.Interop.WindowNative.GetWindowHandle(_hiddenWindow);
        Win32Interop.ShowWindow(_hwnd, Win32Interop.SW_HIDE);

        Settings = new SettingsService();
        Settings.Load();
        Api = new ApiClient(Settings.BackendUrl);
        Auth = new AuthService();
        Clipboard = new ClipboardService();
        Injector = new TextInjector(Clipboard);
        Hotkey = new HotkeyService();

        // Install a WndProc subclass on the hidden window so WM_HOTKEY messages
        // reach HotkeyService.ProcessHotkeyMessage.  The delegate MUST be stored
        // in _subclassProc (a field) so the GC never collects it while it is active.
        _subclassProc = HiddenWindowSubclassProc;
        Win32Interop.SetWindowSubclass(_hwnd, _subclassProc, (UIntPtr)1, UIntPtr.Zero);

        // Register the global hotkey using saved settings
        bool registered = Hotkey.Register(
            _hwnd,
            (uint)Settings.HotkeyModifiers,
            (uint)Settings.HotkeyKey);

        if (!registered)
        {
            DRLogger.Log(
                $"Hotkey registration failed (modifiers=0x{Settings.HotkeyModifiers:X} vk=0x{Settings.HotkeyKey:X})",
                DRLogger.Category.HOTKEY);
        }
        else
        {
            DRLogger.Log(
                $"Hotkey registered (modifiers=0x{Settings.HotkeyModifiers:X} vk=0x{Settings.HotkeyKey:X})",
                DRLogger.Category.HOTKEY);
        }

        // Wire up the hotkey handler
        Hotkey.HotkeyPressed += async (_, _) => await HandleHotkeyAsync();

        // Restore saved session or auto-login for testing
        if (Auth.RestoreSession())
        {
            Api.SetToken(Auth.AccessToken!);
        }
        else
        {
            _ = AutoLoginAsync();
        }

        // Create the loading indicator on the WinUI UI thread before starting the tray thread.
        // InvokeRequired/Invoke will marshal show/hide calls back to this thread as needed.
        _loadingIndicator = new LoadingIndicator();

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

    // ── WndProc subclass ────────────────────────────────────

    /// <summary>
    /// Window procedure subclass installed on the hidden HWND.
    /// Routes WM_HOTKEY to HotkeyService; all other messages fall through to the default.
    /// </summary>
    private IntPtr HiddenWindowSubclassProc(
        IntPtr hWnd, uint uMsg, IntPtr wParam, IntPtr lParam,
        UIntPtr uIdSubclass, UIntPtr dwRefData)
    {
        if (uMsg == HotkeyService.WM_HOTKEY)
        {
            Hotkey.ProcessHotkeyMessage(wParam.ToInt32());
            return IntPtr.Zero;
        }

        return Win32Interop.DefSubclassProc(hWnd, uMsg, wParam, lParam);
    }

    // ── Hotkey handler ──────────────────────────────────────

    /// <summary>
    /// Called (on the WndProc thread) when the global hotkey fires.
    /// Captures selected text from the foreground app and opens the RewritePanel.
    /// </summary>
    private async Task HandleHotkeyAsync()
    {
        if (!Auth.IsLoggedIn)
        {
            DRLogger.Log("Hotkey fired but user is not logged in — ignoring.", DRLogger.Category.HOTKEY);
            return;
        }

        // Capture the foreground window BEFORE we do anything else.
        // After Ctrl+C and panel activation the foreground shifts — we need
        // the original window handle to paste back into later.
        _sourceWindow = Win32Interop.GetForegroundWindow();

        DRLogger.Log($"Hotkey fired — capturing selection from HWND 0x{_sourceWindow:X}", DRLogger.Category.HOTKEY);

        var text = await Clipboard.GetSelectedTextAsync();

        if (string.IsNullOrWhiteSpace(text))
        {
            DRLogger.Log("No text selected — ignoring hotkey.", DRLogger.Category.HOTKEY);
            return;
        }

        var mode = Settings.AppMode;
        DRLogger.Log($"Captured {text.Length} chars — mode={mode.ApiValue()}.", DRLogger.Category.HOTKEY);

        if (mode == AppMode.OneClick)
        {
            await RunOneClickRewriteAsync(text);
        }
        else
        {
            // Must open the WinUI window on the UI (dispatcher) thread
            _dispatcherQueue?.TryEnqueue(() =>
            {
                try
                {
                    DRLogger.Log("Panel: dispatcher entered", DRLogger.Category.PANEL);
                    if (_rewritePanel == null)
                    {
                        DRLogger.Log("Panel: constructing new RewritePanel", DRLogger.Category.PANEL);
                        _rewritePanel = new RewritePanel();
                        DRLogger.Log("Panel: constructed", DRLogger.Category.PANEL);

                        _rewritePanel.ViewModel.PasteRequested += async (_, rewrittenText) =>
                        {
                            _rewritePanel.Close();
                            _rewritePanel = null;
                            await Injector.InjectTextAsync(rewrittenText, _sourceWindow);
                            DRLogger.Log("Paste complete.", DRLogger.Category.HOTKEY);
                        };
                        _rewritePanel.ViewModel.CloseRequested += (_, _) => { _rewritePanel = null; };
                    }

                    DRLogger.Log($"Panel: ShowForText({text.Length} chars)", DRLogger.Category.PANEL);
                    _rewritePanel.ShowForText(text);
                    DRLogger.Log("Panel: ShowForText returned", DRLogger.Category.PANEL);
                }
                catch (Exception ex)
                {
                    DRLogger.Log($"Panel: EXCEPTION {ex.GetType().Name}: {ex.Message}", DRLogger.Category.PANEL);
                    DRLogger.Log($"Panel: stack:\n{ex}", DRLogger.Category.PANEL);
                    _rewritePanel = null;
                }
            });
        }
    }

    private async Task RunOneClickRewriteAsync(string text)
    {
        var tone = Settings.OneClickTone;
        DRLogger.Log($"One-Click rewrite: tone={tone} textlen={text.Length}", DRLogger.Category.HOTKEY);

        ShowLoadingIndicatorSafe();

        try
        {
            var response = await Api.RewriteAsync(text, tone, Settings.TranslateLanguage);
            var rewritten = response?.RewrittenText;

            if (string.IsNullOrEmpty(rewritten))
            {
                DRLogger.Log("One-Click rewrite: empty result from backend.", DRLogger.Category.HOTKEY);
                ShowOneClickError("Empty result from backend");
                return;
            }

            DRLogger.Log("One-Click rewrite OK, pasting via TextInjector.", DRLogger.Category.HOTKEY);
            await Injector.InjectTextAsync(rewritten, _sourceWindow);
        }
        catch (Exception ex)
        {
            DRLogger.Log($"One-Click rewrite FAILED: {ex.Message}", DRLogger.Category.HOTKEY);
            ShowOneClickError(ex.Message);
        }
        finally
        {
            HideLoadingIndicatorSafe();
        }
    }

    private void ShowLoadingIndicatorSafe()
    {
        if (_loadingIndicator == null) return;
        if (_loadingIndicator.InvokeRequired)
        {
            _loadingIndicator.Invoke(new Action(() => _loadingIndicator.ShowAtCursor()));
        }
        else
        {
            _loadingIndicator.ShowAtCursor();
        }
    }

    private void HideLoadingIndicatorSafe()
    {
        if (_loadingIndicator == null) return;
        if (_loadingIndicator.InvokeRequired)
        {
            _loadingIndicator.Invoke(new Action(() => _loadingIndicator.HideIndicator()));
        }
        else
        {
            _loadingIndicator.HideIndicator();
        }
    }

    private void ShowOneClickError(string message)
    {
        DRLogger.Log($"One-Click error: {message}", DRLogger.Category.HOTKEY);

        try
        {
            if (_trayIcon != null)
            {
                _trayIcon.BalloonTipTitle = "DraftRight \u2014 One-Click Rewrite Failed";
                _trayIcon.BalloonTipText = message;
                _trayIcon.BalloonTipIcon = WinForms.ToolTipIcon.Error;
                _trayIcon.ShowBalloonTip(4000);
            }
        }
        catch
        {
            // Best-effort: tray icon may be disposed or BalloonTip unavailable — log only.
        }
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

        // Remove the WndProc subclass before the window is destroyed
        if (_subclassProc != null && _hwnd != IntPtr.Zero)
            Win32Interop.RemoveWindowSubclass(_hwnd, _subclassProc, (UIntPtr)1);

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

// ── Tabbed WinForms Settings Form builder ──
internal static class SettingsFormBuilder
{
    // ── Dark theme constants ─────────────────────────────────
    private static readonly Color BgDark = Color.FromArgb(15, 23, 42);
    private static readonly Color CardBg = Color.FromArgb(30, 41, 59);
    private static readonly Color BrandBlue = Color.FromArgb(93, 135, 255);
    private static readonly Color TextPrimary = Color.FromArgb(226, 232, 240);
    private static readonly Color TextMuted = Color.FromArgb(148, 163, 184);
    private static readonly Color ErrorRed = Color.FromArgb(239, 68, 68);
    private static readonly Color SuccessGreen = Color.FromArgb(34, 197, 94);
    private static readonly Color BorderColor = Color.FromArgb(51, 65, 85);

    public static WinForms.Form Create()
    {
        var form = new WinForms.Form
        {
            Text = "DraftRight Settings",
            Width = 520,
            Height = 560,
            StartPosition = WinForms.FormStartPosition.CenterScreen,
            BackColor = BgDark,
            ForeColor = TextPrimary,
            FormBorderStyle = WinForms.FormBorderStyle.FixedSingle,
            MaximizeBox = false,
        };

        SetFormIcon(form);

        var tabControl = new WinForms.TabControl
        {
            Dock = WinForms.DockStyle.Fill,
            BackColor = BgDark,
            ForeColor = TextPrimary,
            Appearance = WinForms.TabAppearance.Normal,
            SizeMode = WinForms.TabSizeMode.FillToRight,
            Padding = new System.Drawing.Point(12, 6),
        };

        tabControl.TabPages.Add(BuildGeneralTab());
        tabControl.TabPages.Add(BuildRewriteTab());
        tabControl.TabPages.Add(BuildTriggerTab());
        tabControl.TabPages.Add(BuildAccountTab());
        tabControl.TabPages.Add(BuildAdvancedTab());

        form.Controls.Add(tabControl);
        return form;
    }

    // ── Helpers ──────────────────────────────────────────────

    private static void SetFormIcon(WinForms.Form form)
    {
        var exePath = Environment.ProcessPath;
        if (exePath != null)
        {
            var icoPath = System.IO.Path.Combine(
                System.IO.Path.GetDirectoryName(exePath)!, "Assets", "DraftRight.ico");
            if (System.IO.File.Exists(icoPath))
                form.Icon = new Icon(icoPath);
        }
    }

    private static WinForms.TabPage MakeTab(string title)
    {
        return new WinForms.TabPage(title)
        {
            BackColor = BgDark,
            ForeColor = TextPrimary,
            Padding = new WinForms.Padding(16),
            AutoScroll = true,
        };
    }

    private static WinForms.Label MakeSectionHeader(string text, int y)
    {
        return new WinForms.Label
        {
            Text = text,
            Font = new Font("Segoe UI", 12, FontStyle.Bold),
            ForeColor = TextPrimary,
            Location = new Point(16, y),
            AutoSize = true,
        };
    }

    private static WinForms.Label MakeFieldLabel(string text, int y)
    {
        return new WinForms.Label
        {
            Text = text,
            Font = new Font("Segoe UI", 9),
            ForeColor = TextMuted,
            Location = new Point(16, y),
            AutoSize = true,
        };
    }

    private static WinForms.TextBox MakeTextBox(int y, string value = "")
    {
        return new WinForms.TextBox
        {
            Text = value,
            Location = new Point(16, y),
            Size = new Size(448, 30),
            BackColor = CardBg,
            ForeColor = TextPrimary,
            BorderStyle = WinForms.BorderStyle.FixedSingle,
            Font = new Font("Segoe UI", 10),
        };
    }

    private static WinForms.ComboBox MakeComboBox(int y)
    {
        return new WinForms.ComboBox
        {
            Location = new Point(16, y),
            Size = new Size(448, 30),
            BackColor = CardBg,
            ForeColor = TextPrimary,
            Font = new Font("Segoe UI", 10),
            DropDownStyle = WinForms.ComboBoxStyle.DropDownList,
            FlatStyle = WinForms.FlatStyle.Flat,
        };
    }

    private static WinForms.Button MakePrimaryButton(string text, int y, int width = 448)
    {
        var btn = new WinForms.Button
        {
            Text = text,
            Location = new Point(16, y),
            Size = new Size(width, 36),
            BackColor = BrandBlue,
            ForeColor = Color.White,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
        };
        btn.FlatAppearance.BorderSize = 0;
        return btn;
    }

    private static WinForms.Button MakeSecondaryButton(string text, int x, int y, int width = 160)
    {
        var btn = new WinForms.Button
        {
            Text = text,
            Location = new Point(x, y),
            Size = new Size(width, 32),
            BackColor = CardBg,
            ForeColor = TextPrimary,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 10),
        };
        btn.FlatAppearance.BorderColor = BorderColor;
        return btn;
    }

    private static WinForms.CheckBox MakeCheckBox(string text, bool checkedInit, int y)
    {
        return new WinForms.CheckBox
        {
            Text = text,
            Checked = checkedInit,
            ForeColor = TextPrimary,
            BackColor = BgDark,
            Font = new Font("Segoe UI", 10),
            Location = new Point(16, y),
            AutoSize = true,
        };
    }

    // ── Tab builders ─────────────────────────────────────────

    private static WinForms.TabPage BuildGeneralTab()
    {
        var tab = MakeTab("General");
        int y = 16;

        // Backend Server section
        tab.Controls.Add(MakeSectionHeader("Backend Server", y));
        y += 30;
        tab.Controls.Add(MakeFieldLabel("Backend URL", y));
        y += 18;
        var urlBox = MakeTextBox(y, App.Settings.BackendUrl ?? "");
        urlBox.TextChanged += (_, _) =>
        {
            App.Settings.BackendUrl = urlBox.Text.Trim();
            App.Settings.Save();
        };
        tab.Controls.Add(urlBox);
        y += 44;

        // General section
        tab.Controls.Add(MakeSectionHeader("General", y));
        y += 30;
        var autoStart = MakeCheckBox("Launch at Login", App.Settings.AutoStart, y);
        autoStart.CheckedChanged += (_, _) =>
        {
            App.Settings.AutoStart = autoStart.Checked;
            App.Settings.Save();
        };
        tab.Controls.Add(autoStart);
        y += 34;

        // Updates section
        tab.Controls.Add(MakeSectionHeader("Updates", y));
        y += 30;
        tab.Controls.Add(new WinForms.Label
        {
            Text = "Version: 1.0.0",
            ForeColor = TextMuted,
            Font = new Font("Segoe UI", 9),
            Location = new Point(16, y),
            AutoSize = true,
        });
        y += 24;
        var updateBtn = MakeSecondaryButton("Check for Updates", 16, y, 180);
        updateBtn.Click += async (_, _) =>
        {
            updateBtn.Enabled = false;
            updateBtn.Text = "Checking...";
            if (App.UpdateService != null)
                await App.UpdateService.CheckNowAsync();
            updateBtn.Text = "Check for Updates";
            updateBtn.Enabled = true;
        };
        tab.Controls.Add(updateBtn);

        return tab;
    }

    private static WinForms.TabPage BuildRewriteTab()
    {
        var tab = MakeTab("Rewrite");
        int y = 16;

        // Mode section
        tab.Controls.Add(MakeSectionHeader("Mode", y));
        y += 30;
        tab.Controls.Add(MakeFieldLabel("Interaction Mode", y));
        y += 18;
        var modeCombo = MakeComboBox(y);
        modeCombo.Items.Add("Advanced");
        modeCombo.Items.Add("One-Click");
        modeCombo.SelectedIndex = App.Settings.AppMode == AppMode.OneClick ? 1 : 0;
        tab.Controls.Add(modeCombo);
        y += 44;

        // One-Click Tone (conditionally visible)
        var oneClickLabel = MakeFieldLabel("One-Click Tone", y);
        tab.Controls.Add(oneClickLabel);
        y += 18;
        var oneClickCombo = MakeComboBox(y);
        var allTones = Enum.GetValues<Tone>();
        foreach (var t in allTones)
            oneClickCombo.Items.Add(t.DisplayName());
        // Set initial selection
        int initialIdx = 0;
        for (int i = 0; i < allTones.Length; i++)
        {
            if (allTones[i].ApiValue() == App.Settings.OneClickTone)
            {
                initialIdx = i;
                break;
            }
        }
        oneClickCombo.SelectedIndex = initialIdx;
        tab.Controls.Add(oneClickCombo);
        int oneClickBottomY = y + 44;

        // Visibility helper
        void UpdateOneClickVisibility()
        {
            bool visible = modeCombo.SelectedIndex == 1;
            oneClickLabel.Visible = visible;
            oneClickCombo.Visible = visible;
        }
        UpdateOneClickVisibility();

        modeCombo.SelectedIndexChanged += (_, _) =>
        {
            App.Settings.AppMode = modeCombo.SelectedIndex == 1 ? AppMode.OneClick : AppMode.Advanced;
            App.Settings.Save();
            UpdateOneClickVisibility();
        };
        oneClickCombo.SelectedIndexChanged += (_, _) =>
        {
            if (oneClickCombo.SelectedIndex >= 0 && oneClickCombo.SelectedIndex < allTones.Length)
            {
                App.Settings.OneClickTone = allTones[oneClickCombo.SelectedIndex].ApiValue();
                App.Settings.Save();
            }
        };

        y = oneClickBottomY;

        // Panel Tones section
        tab.Controls.Add(MakeSectionHeader("Panel Tones", y));
        y += 30;
        foreach (var tone in allTones)
        {
            var cb = MakeCheckBox(
                $"{tone.Icon()}  {tone.DisplayName()}",
                App.Settings.EnabledTones.Contains(tone.ApiValue()),
                y);
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
            tab.Controls.Add(cb);
            y += 26;
        }
        y += 12;

        // Default Tone (auto-run)
        tab.Controls.Add(MakeFieldLabel("Default Tone (auto-run)", y));
        y += 18;
        var defaultCombo = MakeComboBox(y);
        defaultCombo.Items.Add("(None)");
        int defaultSelected = 0;
        foreach (var tone in allTones)
        {
            defaultCombo.Items.Add(tone.DisplayName());
            if (tone.ApiValue() == App.Settings.DefaultTone)
                defaultSelected = defaultCombo.Items.Count - 1;
        }
        defaultCombo.SelectedIndex = defaultSelected;
        defaultCombo.SelectedIndexChanged += (_, _) =>
        {
            if (defaultCombo.SelectedIndex == 0)
                App.Settings.DefaultTone = "";
            else
                App.Settings.DefaultTone = allTones[defaultCombo.SelectedIndex - 1].ApiValue();
            App.Settings.Save();
        };
        tab.Controls.Add(defaultCombo);
        y += 44;

        // Translation section
        tab.Controls.Add(MakeSectionHeader("Translation", y));
        y += 30;
        tab.Controls.Add(MakeFieldLabel("Target Language", y));
        y += 18;
        var langBox = MakeTextBox(y, App.Settings.TranslateLanguage);
        langBox.TextChanged += (_, _) =>
        {
            App.Settings.TranslateLanguage = langBox.Text.Trim();
            App.Settings.Save();
        };
        tab.Controls.Add(langBox);

        return tab;
    }

    private static WinForms.TabPage BuildTriggerTab()
    {
        var tab = MakeTab("Trigger");
        int y = 16;

        tab.Controls.Add(MakeSectionHeader("Hotkey", y));
        y += 30;

        tab.Controls.Add(MakeFieldLabel("Current Hotkey", y));
        y += 18;

        tab.Controls.Add(new WinForms.Label
        {
            Text = FormatHotkey(App.Settings.HotkeyModifiers, App.Settings.HotkeyKey),
            ForeColor = BrandBlue,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
            Location = new Point(16, y),
            AutoSize = true,
        });
        y += 32;

        tab.Controls.Add(new WinForms.Label
        {
            Text = "Select text anywhere, then press the hotkey to rewrite.\r\n" +
                   "Hotkey editing is not yet available in this build.",
            ForeColor = TextMuted,
            Font = new Font("Segoe UI", 9),
            Location = new Point(16, y),
            Size = new Size(448, 40),
        });

        return tab;
    }

    private static string FormatHotkey(int mods, int key)
    {
        var parts = new List<string>();
        if ((mods & 0x0002) != 0) parts.Add("Ctrl");
        if ((mods & 0x0004) != 0) parts.Add("Shift");
        if ((mods & 0x0001) != 0) parts.Add("Alt");
        if ((mods & 0x0008) != 0) parts.Add("Win");
        var keyChar = key >= 0x41 && key <= 0x5A
            ? ((char)key).ToString()
            : $"Key0x{key:X2}";
        parts.Add(keyChar);
        return string.Join(" + ", parts);
    }

    private static WinForms.TabPage BuildAccountTab()
    {
        var tab = MakeTab("Account");
        int y = 16;

        var signedIn = !string.IsNullOrEmpty(App.Auth.AccessToken);

        if (signedIn)
        {
            tab.Controls.Add(MakeSectionHeader("Signed In", y));
            y += 30;

            var emailDisplay = App.Auth.CurrentEmail;
            tab.Controls.Add(new WinForms.Label
            {
                Text = string.IsNullOrEmpty(emailDisplay)
                    ? "Signed in"
                    : $"Signed in as {emailDisplay}",
                ForeColor = SuccessGreen,
                Font = new Font("Segoe UI", 10),
                Location = new Point(16, y),
                AutoSize = true,
            });
            y += 32;

            var signOutBtn = MakeSecondaryButton("Sign Out", 16, y);
            signOutBtn.Click += (_, _) =>
            {
                App.Auth.ClearTokens();
                App.Api.ClearToken();
                WinForms.MessageBox.Show(
                    "Signed out successfully. Please reopen Settings to sign in again.",
                    "DraftRight",
                    WinForms.MessageBoxButtons.OK,
                    WinForms.MessageBoxIcon.Information);
            };
            tab.Controls.Add(signOutBtn);
        }
        else
        {
            tab.Controls.Add(MakeSectionHeader("Sign In", y));
            y += 30;

            tab.Controls.Add(MakeFieldLabel("Email", y));
            y += 18;
            var emailBox = MakeTextBox(y);
            tab.Controls.Add(emailBox);
            y += 44;

            tab.Controls.Add(MakeFieldLabel("Password", y));
            y += 18;
            var passBox = MakeTextBox(y);
            passBox.UseSystemPasswordChar = true;
            tab.Controls.Add(passBox);
            y += 44;

            // Read-only multi-line TextBox styled as a label so the user
            // can select + copy any error message (Ctrl+C, right-click, drag).
            // WinForms.Label.Text isn't selectable; TextBox.Text is.
            var statusBox = new WinForms.TextBox
            {
                ReadOnly = true,
                Multiline = true,
                ForeColor = ErrorRed,
                BackColor = BgDark,
                BorderStyle = WinForms.BorderStyle.None,
                Font = new Font("Segoe UI", 9),
                Location = new Point(16, y),
                Size = new Size(380, 60),
                ScrollBars = WinForms.ScrollBars.Vertical,
                TabStop = false,
                Visible = false,  // hidden until there's a message
            };
            tab.Controls.Add(statusBox);

            // One-click Copy button — hidden until there's text to copy.
            var copyBtn = new WinForms.Button
            {
                Text = "Copy",
                Location = new Point(400, y),
                Size = new Size(64, 24),
                BackColor = CardBg,
                ForeColor = TextMuted,
                FlatStyle = WinForms.FlatStyle.Flat,
                Font = new Font("Segoe UI", 8),
                Visible = false,
                TabStop = false,
            };
            copyBtn.FlatAppearance.BorderColor = BorderColor;
            copyBtn.Click += (_, _) =>
            {
                if (!string.IsNullOrEmpty(statusBox.Text))
                {
                    WinForms.Clipboard.SetText(statusBox.Text);
                    copyBtn.Text = "Copied";
                    var t = new WinForms.Timer { Interval = 1200 };
                    t.Tick += (_, _) => { copyBtn.Text = "Copy"; t.Stop(); t.Dispose(); };
                    t.Start();
                }
            };
            tab.Controls.Add(copyBtn);

            // Helper: show/hide the status box + copy button together.
            Action<string, Color> setStatus = (text, color) =>
            {
                statusBox.Text = text;
                statusBox.ForeColor = color;
                var hasText = !string.IsNullOrEmpty(text);
                statusBox.Visible = hasText;
                // Only show Copy for actual errors (red), not success messages.
                copyBtn.Visible = hasText && color == ErrorRed;
            };

            y += 72;

            var signInBtn = MakePrimaryButton("Sign In", y);
            signInBtn.Click += async (_, _) =>
            {
                setStatus("", ErrorRed);
                signInBtn.Enabled = false;
                try
                {
                    App.Api.SetBaseUrl(App.Settings.BackendUrl ?? "");
                    var result = await App.Api.LoginAsync(emailBox.Text.Trim(), passBox.Text);
                    if (!string.IsNullOrEmpty(result.AccessToken))
                    {
                        App.Auth.SaveTokens(result.AccessToken, result.RefreshToken, result.User?.Email);
                        App.Api.SetToken(result.AccessToken);
                        setStatus("Signed in! Please reopen Settings.", SuccessGreen);
                    }
                    else
                    {
                        setStatus("Login failed.", ErrorRed);
                    }
                }
                catch (Exception ex)
                {
                    setStatus(ex.ToString(), ErrorRed);
                    DRLogger.Log($"Login error: {ex}", DRLogger.Category.AUTH);
                }
                finally
                {
                    signInBtn.Enabled = true;
                }
            };
            tab.Controls.Add(signInBtn);
        }

        return tab;
    }

    private static WinForms.TabPage BuildAdvancedTab()
    {
        var tab = MakeTab("Advanced");
        int y = 16;

        tab.Controls.Add(MakeSectionHeader("Logs", y));
        y += 30;

        // DRLogger has LogFilePath but no IsEnabled — the checkbox is cosmetic
        var loggingEnabled = MakeCheckBox("Enable Logging", true, y);
        loggingEnabled.Enabled = false; // read-only; logging is always-on in this build
        tab.Controls.Add(loggingEnabled);
        y += 34;

        tab.Controls.Add(MakeFieldLabel("Log file location:", y));
        y += 20;

        var logFilePath = DRLogger.LogFilePath;
        var pathBox = MakeTextBox(y, logFilePath);
        pathBox.ReadOnly = true;
        tab.Controls.Add(pathBox);
        y += 44;

        var openBtn = MakeSecondaryButton("Open", 16, y, 80);
        openBtn.Click += (_, _) =>
        {
            if (System.IO.File.Exists(logFilePath))
            {
                System.Diagnostics.Process.Start(
                    new System.Diagnostics.ProcessStartInfo(logFilePath) { UseShellExecute = true });
            }
            else
            {
                WinForms.MessageBox.Show(
                    "Log file does not exist yet.",
                    "DraftRight",
                    WinForms.MessageBoxButtons.OK,
                    WinForms.MessageBoxIcon.Information);
            }
        };
        tab.Controls.Add(openBtn);

        var clearBtn = MakeSecondaryButton("Clear", 108, y, 80);
        clearBtn.Click += (_, _) =>
        {
            try
            {
                System.IO.File.WriteAllText(logFilePath, "");
            }
            catch (Exception ex)
            {
                WinForms.MessageBox.Show(
                    $"Could not clear log: {ex.Message}",
                    "DraftRight",
                    WinForms.MessageBoxButtons.OK,
                    WinForms.MessageBoxIcon.Warning);
            }
        };
        tab.Controls.Add(clearBtn);

        return tab;
    }
}
