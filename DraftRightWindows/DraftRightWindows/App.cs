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
    private WinForms.ToolStripMenuItem? _updateMenuItem;
    // Base tray icon and a cached copy with an "update ready" badge dot. We
    // swap _trayIcon.Icon between them as the update state changes.
    private Icon? _baseTrayIcon;
    private Icon? _badgeTrayIcon;
    // Tracks the last badge state so the "update ready" balloon fires once on
    // the rising edge, not on every AvailableUpdateChanged tick.
    private bool _updateBadgeShown;
    public static BackendStatus CurrentStatus { get; private set; } = BackendStatus.Offline;
    private DateTime _lastAutoRecovery = DateTime.MinValue;
    public static UpdateService? UpdateService { get; private set; }

    /// <summary>True after the backend has explicitly rejected our refresh
    /// token (HTTP 401/403). Cleared on successful re-login. Mirrors the
    /// macOS AppModel.sessionExpired flag.</summary>
    public static bool SessionExpired { get; private set; }
    /// <summary>One-shot guard so the "session expired" alert fires at most
    /// once per session-loss event — the 30-s health-check timer would
    /// otherwise re-trigger it on every cycle.</summary>
    private static bool _didPromptForReauth;

    // ── Rewrite flow ────────────────────────────────────────
    private DispatcherQueue? _dispatcherQueue;
    // The WinUI RewritePanel relies on theme XAML resources (themeresources.xaml,
    // TabViewScrollButtonBackground, etc.) that are only fully resolvable when the
    // app is built with the VS AppX MSBuild tooling that produces a real
    // resources.pri. On unpackaged builds without that tooling, the WinUI panel
    // crashes during the first XAML render with STATUS_STOWED_EXCEPTION (0xc000027b)
    // / "Cannot find a Resource with the Name/Key TabViewScrollButtonBackground".
    //
    // RewritePanelForm is a WinForms reimplementation of the same UI surface
    // (same ViewModel API, same dark theme) that doesn't go through XAML and
    // therefore works reliably on local x64 builds. Used in place of the WinUI
    // panel below.
    private RewritePanelForm? _rewritePanel;
    private Thread? _rewritePanelThread;
    private IntPtr _sourceWindow = IntPtr.Zero;
    private LoadingIndicator? _loadingIndicator;

    // Must be stored as a field — delegate must stay alive for the lifetime of the subclass
    private Win32Interop.SUBCLASSPROC? _subclassProc;

    public App()
    {
        // Capture every kind of unhandled exception with as much context as
        // possible. Each handler logs to BOTH the rolling log file (via
        // DRLogger) and a top-level draftright-crash.log on the Desktop so
        // post-mortem debugging works even if the Local AppData folder is
        // unreachable.

        // Configure the server-side error reporter so the three handlers
        // below send to /errors as well as logging locally. The bearer token
        // provider is resolved lazily — Auth doesn't exist yet at this point,
        // but a crash early in startup is rare and would just send anonymously.
        ErrorReporter.Configure(
            backendUrl: Constants.DefaultBackendUrl,
            bearerTokenProvider: () => Auth?.AccessToken
        );

        UnhandledException += (sender, e) =>
        {
            DRLogger.Error($"WinUI UnhandledException: {e.Exception}", DRLogger.Category.APP);
            WriteCrashFile("WinUI", e.Exception);
            ErrorReporter.Report(e.Exception, source: "WinUI", severity: "fatal");
            e.Handled = true;
        };

        AppDomain.CurrentDomain.UnhandledException += (sender, e) =>
        {
            var ex = e.ExceptionObject as Exception;
            DRLogger.Error($"AppDomain UnhandledException (terminating={e.IsTerminating}): {ex}",
                DRLogger.Category.APP);
            WriteCrashFile("AppDomain", ex);
            if (ex != null) ErrorReporter.Report(ex, source: "AppDomain", severity: "fatal");
        };

        TaskScheduler.UnobservedTaskException += (sender, e) =>
        {
            DRLogger.Warn($"UnobservedTaskException: {e.Exception}", DRLogger.Category.APP);
            WriteCrashFile("UnobservedTask", e.Exception);
            ErrorReporter.Report(e.Exception, source: "UnobservedTask", severity: "error");
            e.SetObserved();
        };

        // One-shot startup banner so every log file starts with the
        // environment fingerprint: app version, .NET, OS, architecture.
        try
        {
            var asm = System.Reflection.Assembly.GetExecutingAssembly();
            var ver = asm.GetName().Version?.ToString() ?? "?";
            var os = Environment.OSVersion.VersionString;
            var bits = Environment.Is64BitProcess ? "64-bit" : "32-bit";
            var arch = System.Runtime.InteropServices.RuntimeInformation.ProcessArchitecture;
            var dotnet = Environment.Version.ToString();
            DRLogger.Log(
                $"DraftRight startup: app={ver} .NET={dotnet} OS={os} {bits} arch={arch}",
                DRLogger.Category.APP);
        }
        catch (Exception ex)
        {
            DRLogger.Warn($"Startup banner failed: {ex.Message}", DRLogger.Category.APP);
        }
    }

    private static void WriteCrashFile(string source, Exception? ex)
    {
        try
        {
            var path = System.IO.Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.Desktop),
                "draftright-crash.log");
            System.IO.File.AppendAllText(path,
                $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss.fff}] {source}: {ex}\n\n");
        }
        catch
        {
            // best-effort — Desktop may not be writable in some elevated contexts
        }
    }

    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
        DRLogger.Log("OnLaunched start", DRLogger.Category.APP);

        // Capture the UI-thread dispatcher queue before any async work
        _dispatcherQueue = DispatcherQueue.GetForCurrentThread();
        DRLogger.Log($"OnLaunched: dispatcher captured={_dispatcherQueue != null}", DRLogger.Category.APP);

        _hiddenWindow = new Window { Title = "DraftRight" };
        DRLogger.Log("OnLaunched: hidden window created", DRLogger.Category.APP);
        _hwnd = WinRT.Interop.WindowNative.GetWindowHandle(_hiddenWindow);
        DRLogger.Log($"OnLaunched: hwnd=0x{_hwnd.ToInt64():X}", DRLogger.Category.APP);
        Win32Interop.ShowWindow(_hwnd, Win32Interop.SW_HIDE);

        Settings = new SettingsService();
        Settings.Load();
        // Mirror the saved logging flag into DRLogger before any further Log()
        // calls so the user's preference takes effect from the next line on.
        DRLogger.IsEnabled = Settings.LoggingEnabled;
        // Heal the run-at-login registration if the setting says it should be on
        // but the Run key is missing (re-points it at the current exe too).
        // Never delete here — so the installer's autostart task isn't undone.
        if (Settings.AutoStart && !StartupRegistration.IsEnabled())
            StartupRegistration.SetEnabled(true);

        // Reconcile the Scheduled Task that supervises the running process.
        // The legacy Run-key registration above only fires at logon; it does
        // nothing if DraftRight dies mid-session. KeepAliveAgent installs a
        // Task Scheduler entry whose RestartOnFailure trigger respawns the
        // app within ~10 s of any abnormal exit (crash, OS kill, etc.). The
        // Run-key registration and the Scheduled Task are intentionally
        // independent so removing one doesn't break the other.
        Services.KeepAliveAgent.Reconcile(desiredRunAtLogon: Settings.AutoStart);
        DRLogger.Log($"OnLaunched: settings loaded — BackendUrl={Settings.BackendUrl} AppMode={Settings.AppMode} Hotkey={Settings.HotkeyModifiers:X}+{Settings.HotkeyKey:X}",
            DRLogger.Category.APP);

        Api = new ApiClient(Settings.BackendUrl);
        Auth = new AuthService();
        Clipboard = new ClipboardService();
        Injector = new TextInjector(Clipboard);
        Hotkey = new HotkeyService();
        DRLogger.Log("OnLaunched: services constructed (Api, Auth, Clipboard, Injector, Hotkey)", DRLogger.Category.APP);

        // Auto-refresh on 401: backend access tokens expire after 15 min; the
        // stored refresh_token (7-day) is exchanged for a fresh pair via
        // /auth/refresh. If refresh itself fails, clear tokens so the user is
        // prompted to sign in again instead of looping on 401s.
        Api.OnUnauthorized = async () =>
        {
            var refreshToken = Auth.RefreshToken;
            if (string.IsNullOrEmpty(refreshToken))
            {
                DRLogger.Log("Auto-refresh: no refresh_token stored — clearing session.", DRLogger.Category.AUTH);
                Auth.ClearTokens();
                Api.ClearToken();
                RaiseSessionExpired();
                return false;
            }
            try
            {
                var result = await Api.RefreshAsync(refreshToken);
                if (string.IsNullOrEmpty(result.AccessToken))
                {
                    DRLogger.Log("Auto-refresh: backend returned empty access_token — clearing session.", DRLogger.Category.AUTH);
                    Auth.ClearTokens();
                    Api.ClearToken();
                    RaiseSessionExpired();
                    return false;
                }
                Auth.SaveTokens(result.AccessToken, result.RefreshToken, Auth.CurrentEmail);
                Api.SetToken(result.AccessToken);
                DRLogger.Log("Auto-refresh: succeeded.", DRLogger.Category.AUTH);
                return true;
            }
            catch (Exception ex)
            {
                DRLogger.Error($"Auto-refresh: failed — {ex.Message}", DRLogger.Category.AUTH);
                Auth.ClearTokens();
                Api.ClearToken();
                RaiseSessionExpired();
                return false;
            }
        };

        // Clear the session-expired guard whenever the user successfully
        // signs in again (or auto-refresh succeeds against a fresh token).
        Auth.TokensSaved += () =>
        {
            SessionExpired = false;
            _didPromptForReauth = false;
        };

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
            DRLogger.Error(
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

        // Start update check — 10 seconds after launch.
        // Read the real assembly version. Previously hardcoded to "1.0.0", which
        // meant every backend release looked newer (the user got an "update
        // available" prompt for versions OLDER than what they had installed).
        var asmVer = System.Reflection.Assembly.GetExecutingAssembly()
            .GetName().Version?.ToString() ?? "0.0.0";
        // Drop trailing ".0" if present so the local string compares cleanly
        // against semver-style backend versions ("2.1.1" not "2.1.1.0").
        var currentVersion = asmVer.EndsWith(".0") ? asmVer.Substring(0, asmVer.Length - 2) : asmVer;
        DRLogger.Log($"UpdateService current version: {currentVersion} (assembly {asmVer})",
            DRLogger.Category.APP);
        UpdateService = new UpdateService(currentVersion, Settings.BackendUrl);
        _ = Task.Delay(TimeSpan.FromSeconds(10)).ContinueWith(_ => UpdateService.CheckIfNeededAsync());

        // Show a one-time "What's New" notice if we just updated to a newer
        // version since the last run.
        _ = MaybeShowWhatsNewAsync(currentVersion);
    }

    /// <summary>
    /// On the first launch after an update, shows the release notes the backend
    /// advertises for the now-running version. Compares the running version
    /// against the last one we recorded; only fires on an actual upgrade (not a
    /// fresh install or downgrade), and records the current version so it shows
    /// at most once per update.
    /// </summary>
    private async Task MaybeShowWhatsNewAsync(string currentVersion)
    {
        try
        {
            var lastSeen = Settings.LastSeenVersion;
            // Always record the current version so the notice can't repeat.
            if (lastSeen != currentVersion)
            {
                Settings.LastSeenVersion = currentVersion;
                Settings.Save();
            }

            // Fresh install (no prior version) or not an upgrade → nothing to show.
            var svc = UpdateService;
            if (svc is null || string.IsNullOrEmpty(lastSeen)
                || !UpdateService.IsNewerForTest(currentVersion, lastSeen))
                return;

            DRLogger.Log($"Post-update: detected upgrade {lastSeen} -> {currentVersion}, fetching What's New", DRLogger.Category.APP);
            var notes = await svc.GetReleaseNotesForVersionAsync(currentVersion);
            if (string.IsNullOrWhiteSpace(notes))
            {
                DRLogger.Log($"Post-update: no release notes available for {currentVersion} — skipping notice", DRLogger.Category.APP);
                return;
            }
            Views.WhatsNewWindow.Show(currentVersion, notes!);
        }
        catch (Exception ex)
        {
            DRLogger.Warn($"Post-update What's New failed: {ex.Message}", DRLogger.Category.APP);
        }
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
            ShowRewritePanelOnNewThread(text);
        }
    }

    /// <summary>
    /// Runs the WinForms RewritePanelForm on a dedicated STA thread with its own
    /// message pump (Application.Run). When the panel closes, the thread exits.
    /// </summary>
    private void ShowRewritePanelOnNewThread(string text)
    {
        // If a previous panel is still open, just bring it forward and update its text.
        if (_rewritePanel != null)
        {
            try
            {
                _rewritePanel.BeginInvoke(new Action(() =>
                {
                    _rewritePanel.SetInputText(text);
                    _rewritePanel.Activate();
                }));
                return;
            }
            catch
            {
                _rewritePanel = null;
            }
        }

        var sourceHwnd = _sourceWindow;
        _rewritePanelThread = new Thread(() =>
        {
            try
            {
                DRLogger.Log("Panel: thread starting", DRLogger.Category.PANEL);
                using var panel = new RewritePanelForm();
                _rewritePanel = panel;
                panel.SetInputText(text);

                panel.ViewModel.PasteRequested += (_, rewrittenText) =>
                {
                    // Hide panel immediately so focus can return to source app.
                    // Close (dispose) AFTER inject completes — otherwise the panel's
                    // STA pump tears down and the await Task.Delay continuations
                    // inside Injector.InjectTextAsync get dropped, leaving SendInput
                    // unfired.
                    if (panel.IsHandleCreated)
                    {
                        try { panel.BeginInvoke(new Action(() => panel.Hide())); }
                        catch { /* panel may already be closing */ }
                    }
                    _ = Task.Run(async () =>
                    {
                        try
                        {
                            await Injector.InjectTextAsync(rewrittenText, sourceHwnd);
                            DRLogger.Log("Paste complete.", DRLogger.Category.HOTKEY);
                        }
                        catch (Exception ex)
                        {
                            DRLogger.Error($"Paste failed: {ex.Message}", DRLogger.Category.HOTKEY);
                        }
                        finally
                        {
                            try
                            {
                                if (panel.IsHandleCreated)
                                    panel.BeginInvoke(new Action(() => panel.Close()));
                            }
                            catch { /* panel may already be gone */ }
                        }
                    });
                };

                DRLogger.Log("Panel: running message loop", DRLogger.Category.PANEL);
                System.Windows.Forms.Application.Run(panel);
                DRLogger.Log("Panel: closed", DRLogger.Category.PANEL);
            }
            catch (Exception ex)
            {
                DRLogger.Error($"Panel: EXCEPTION {ex.GetType().Name}: {ex.Message}", DRLogger.Category.PANEL);
            }
            finally
            {
                _rewritePanel = null;
            }
        });
        _rewritePanelThread.SetApartmentState(ApartmentState.STA);
        _rewritePanelThread.IsBackground = true;
        _rewritePanelThread.Start();
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
            DRLogger.Error($"One-Click rewrite FAILED: {ex.Message}", DRLogger.Category.HOTKEY);
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
        DRLogger.Error($"One-Click error: {message}", DRLogger.Category.HOTKEY);

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
                _baseTrayIcon = new Icon(icoPath);
            else
                _baseTrayIcon = (Icon)SystemIcons.Application.Clone();
        }
        else
        {
            _baseTrayIcon = (Icon)SystemIcons.Application.Clone();
        }
        _trayIcon.Icon = _baseTrayIcon;
        // Pre-build the badged variant once so RefreshUpdateMenuItem can swap
        // it in without recompositing on every update-state change.
        try { _badgeTrayIcon = TrayIconBadge.WithDot(_baseTrayIcon); }
        catch (Exception ex) { DRLogger.Warn($"Tray badge icon build failed: {ex.Message}", DRLogger.Category.APP); }

        var menu = new WinForms.ContextMenuStrip();
        _statusMenuItem = new WinForms.ToolStripMenuItem("Offline") { Enabled = false };
        menu.Items.Add(_statusMenuItem);
        _updateMenuItem = new WinForms.ToolStripMenuItem("Update available")
        {
            Visible = false,
            ForeColor = Color.FromArgb(34, 197, 94),
        };
        _updateMenuItem.Click += (_, _) =>
        {
            var u = UpdateService?.AvailableUpdate;
            if (u != null) UpdateService!.StartInstall(u);
        };
        menu.Items.Add(_updateMenuItem);
        menu.Items.Add(new WinForms.ToolStripSeparator());
        menu.Items.Add("Settings", null, (_, _) => OpenSettings());
        menu.Items.Add(new WinForms.ToolStripSeparator());
        menu.Items.Add("Sign Out", null, (_, _) => { Auth.ClearTokens(); Api.ClearToken(); });
        menu.Items.Add("Quit", null, (_, _) => DoQuit());

        _trayIcon.ContextMenuStrip = menu;
        _trayIcon.DoubleClick += (_, _) => OpenSettings();
        _trayIcon.Visible = true;

        // Reflect "update available" in the tray menu — both now (in case the
        // 10s-after-launch check already finished) and whenever it changes.
        if (UpdateService != null)
        {
            UpdateService.AvailableUpdateChanged += () =>
            {
                try { menu.BeginInvoke(new Action(RefreshUpdateMenuItem)); }
                catch { /* tray menu gone */ }
            };
            RefreshUpdateMenuItem();
        }

        WinForms.Application.Run();
    }

    // Show/hide + relabel the tray "update available" item from the current
    // UpdateService.AvailableUpdate. Must run on the tray thread.
    private void RefreshUpdateMenuItem()
    {
        if (_updateMenuItem == null) return;
        var u = UpdateService?.AvailableUpdate;
        var staged = u != null && (UpdateService?.UpdateStaged ?? false);
        if (u != null)
        {
            _updateMenuItem.Text = staged
                ? $"Update {u.Version} ready — restart & install"
                : $"Update {u.Version} available — install now";
            _updateMenuItem.Visible = true;
        }
        else
        {
            _updateMenuItem.Visible = false;
        }

        UpdateTrayBadge(u != null, u?.Version, staged);
    }

    /// <summary>
    /// Reflects "an update is ready to install" on the always-visible tray
    /// icon: swaps in the badged icon and, on the rising edge, shows a one-time
    /// balloon so the badge isn't silently easy to miss. Must run on the tray
    /// thread (called from <see cref="RefreshUpdateMenuItem"/>).
    /// </summary>
    private void UpdateTrayBadge(bool hasUpdate, string? version, bool staged)
    {
        if (_trayIcon == null) return;
        try
        {
            // Fall back to the base icon if badge compositing failed at startup.
            _trayIcon.Icon = (hasUpdate && _badgeTrayIcon != null) ? _badgeTrayIcon : _baseTrayIcon;

            if (hasUpdate && !_updateBadgeShown)
            {
                _trayIcon.BalloonTipTitle = "DraftRight update";
                _trayIcon.BalloonTipText = staged
                    ? $"Version {version} is ready to install — open the DraftRight tray menu to restart & install."
                    : $"Version {version} is available — open the DraftRight tray menu to install.";
                _trayIcon.BalloonTipIcon = WinForms.ToolTipIcon.Info;
                _trayIcon.ShowBalloonTip(5000);
            }
        }
        catch { /* tray icon may be disposed during shutdown */ }
        _updateBadgeShown = hasUpdate;
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
        _badgeTrayIcon?.Dispose();
        _badgeTrayIcon = null;
        _baseTrayIcon?.Dispose();
        _baseTrayIcon = null;
        _settingsForm?.Close();
        WinForms.Application.ExitThread();
        Environment.Exit(0);
    }

    /// <summary>
    /// Pops a one-shot "Session expired — please sign in" alert when the
    /// backend rejects our refresh token. Guarded by <c>_didPromptForReauth</c>
    /// so the 30-s health-check timer can't re-trigger it. Fires the dialog on
    /// a worker thread (MessageBox runs its own modal loop) to avoid blocking
    /// the OnUnauthorized callback. Clicking "Open Settings" surfaces the
    /// existing sign-in UI; "Cancel" leaves the app silent until the user
    /// opens Settings themselves.
    /// </summary>
    private static void RaiseSessionExpired()
    {
        if (_didPromptForReauth) return;
        _didPromptForReauth = true;
        SessionExpired = true;
        DRLogger.Log("Session expired — surfacing sign-in prompt.", DRLogger.Category.AUTH);

        Task.Run(() =>
        {
            try
            {
                var result = WinForms.MessageBox.Show(
                    "Your DraftRight session has expired. Please sign in again to keep using rewrite.",
                    "DraftRight — Session expired",
                    WinForms.MessageBoxButtons.OKCancel,
                    WinForms.MessageBoxIcon.Warning);

                if (result == WinForms.DialogResult.OK
                    && Application.Current is App app)
                {
                    app._dispatcherQueue?.TryEnqueue(() => app.OpenSettings());
                }
            }
            catch (Exception ex)
            {
                DRLogger.Warn($"RaiseSessionExpired alert failed: {ex.Message}", DRLogger.Category.AUTH);
            }
        });
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
        // Reflect the actual registry state (the installer's autostart task or a
        // prior toggle may have set it), not just the saved bool.
        var autoStart = MakeCheckBox("Launch at Login", StartupRegistration.IsEnabled(), y);
        autoStart.CheckedChanged += (_, _) =>
        {
            StartupRegistration.SetEnabled(autoStart.Checked);
            App.Settings.AutoStart = autoStart.Checked;
            App.Settings.Save();
            // Mirror the toggle in the Scheduled Task that supervises the
            // running process. Toggle ON installs (logon launch + crash
            // respawn). Toggle OFF removes the task entirely.
            Services.KeepAliveAgent.Reconcile(desiredRunAtLogon: autoStart.Checked);
        };
        tab.Controls.Add(autoStart);
        y += 34;

        // Updates section
        tab.Controls.Add(MakeSectionHeader("Updates", y));
        y += 30;
        var asmVer = System.Reflection.Assembly.GetExecutingAssembly().GetName().Version?.ToString() ?? "?";
        var displayVer = asmVer.EndsWith(".0") ? asmVer.Substring(0, asmVer.Length - 2) : asmVer;
        tab.Controls.Add(new WinForms.Label
        {
            Text = $"Version: {displayVer}",
            ForeColor = TextMuted,
            Font = new Font("Segoe UI", 9),
            Location = new Point(16, y),
            AutoSize = true,
        });
        y += 24;

        // "Update X.Y.Z available — click here to download and install" link.
        // Hidden when up to date. Driven by UpdateService.AvailableUpdate so
        // it reflects the background check without the user pressing the
        // button below.
        var updateLink = new WinForms.LinkLabel
        {
            AutoSize = true,
            Location = new Point(16, y),
            Font = new Font("Segoe UI", 9, FontStyle.Bold),
            LinkColor = Color.FromArgb(96, 165, 250),
            ActiveLinkColor = Color.FromArgb(147, 197, 253),
            Visible = false,
        };
        updateLink.LinkClicked += (_, _) =>
        {
            var u = App.UpdateService?.AvailableUpdate;
            if (u != null) App.UpdateService!.StartInstall(u);
        };
        tab.Controls.Add(updateLink);
        y += 26;

        void RefreshUpdateLink()
        {
            var u = App.UpdateService?.AvailableUpdate;
            if (u != null)
            {
                var staged = App.UpdateService?.UpdateStaged ?? false;
                updateLink.Text = staged
                    ? $"Update {u.Version} downloaded — click here to restart and install"
                    : $"Update {u.Version} available — click here to download and install";
                updateLink.Visible = true;
            }
            else
            {
                updateLink.Visible = false;
            }
        }
        RefreshUpdateLink();
        // Re-poll in the background so a newly-published update appears here
        // even if the 10s-after-launch check ran before it went live.
        if (App.UpdateService != null)
        {
            _ = App.UpdateService.RefreshAvailableUpdateAsync().ContinueWith(_ =>
            {
                try
                {
                    if (!updateLink.IsDisposed)
                        updateLink.BeginInvoke(new Action(RefreshUpdateLink));
                }
                catch { /* settings window closed */ }
            });
        }

        var updateBtn = MakeSecondaryButton("Check for Updates", 16, y, 180);
        updateBtn.Click += async (_, _) =>
        {
            updateBtn.Enabled = false;
            updateBtn.Text = "Checking...";
            if (App.UpdateService != null)
                await App.UpdateService.CheckNowAsync();
            RefreshUpdateLink();
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
        var allTones = Enum.GetValues<Tone>();

        // ── Mode section (always visible) ───────────────────
        tab.Controls.Add(MakeSectionHeader("Mode", y));
        y += 30;
        tab.Controls.Add(MakeFieldLabel("Interaction Mode", y));
        y += 18;
        var modeCombo = MakeComboBox(y);
        modeCombo.Items.Add(AppMode.Advanced.DisplayName());
        modeCombo.Items.Add(AppMode.OneClick.DisplayName());
        modeCombo.SelectedIndex = App.Settings.AppMode == AppMode.OneClick ? 1 : 0;
        tab.Controls.Add(modeCombo);
        y += 44;

        // ── Simple + Advanced blocks share the same starting Y ──
        // Only one block is visible at a time; the other is hidden so the
        // user never sees an empty gap between Mode and the visible block.
        int conditionalY = y;

        var simpleOnlyControls = new List<WinForms.Control>();
        var advancedOnlyControls = new List<WinForms.Control>();

        // ── Simple block: Simple Tone + Default Tone ────────
        int sy = conditionalY;
        var oneClickLabel = MakeFieldLabel($"{AppMode.OneClick.DisplayName()} Tone", sy);
        tab.Controls.Add(oneClickLabel); simpleOnlyControls.Add(oneClickLabel);
        sy += 18;
        var oneClickCombo = MakeComboBox(sy);
        foreach (var t in allTones) oneClickCombo.Items.Add(t.DisplayName());
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
        oneClickCombo.SelectedIndexChanged += (_, _) =>
        {
            if (oneClickCombo.SelectedIndex >= 0 && oneClickCombo.SelectedIndex < allTones.Length)
            {
                App.Settings.OneClickTone = allTones[oneClickCombo.SelectedIndex].ApiValue();
                App.Settings.Save();
            }
        };
        tab.Controls.Add(oneClickCombo); simpleOnlyControls.Add(oneClickCombo);
        sy += 44;

        var defaultToneLabel = MakeFieldLabel("Default Tone (auto-run)", sy);
        tab.Controls.Add(defaultToneLabel); simpleOnlyControls.Add(defaultToneLabel);
        sy += 18;
        var defaultCombo = MakeComboBox(sy);
        int defaultSelected = 0;
        for (int i = 0; i < allTones.Length; i++)
        {
            defaultCombo.Items.Add(allTones[i].DisplayName());
            if (allTones[i].ApiValue() == App.Settings.DefaultTone)
                defaultSelected = i;
        }
        defaultCombo.SelectedIndex = defaultSelected;
        defaultCombo.SelectedIndexChanged += (_, _) =>
        {
            if (defaultCombo.SelectedIndex >= 0 && defaultCombo.SelectedIndex < allTones.Length)
            {
                App.Settings.DefaultTone = allTones[defaultCombo.SelectedIndex].ApiValue();
                App.Settings.Save();
            }
        };
        tab.Controls.Add(defaultCombo); simpleOnlyControls.Add(defaultCombo);
        sy += 30;
        int simpleBlockHeight = sy - conditionalY;

        // ── Advanced block: Panel Tones ─────────────────────
        int ay = conditionalY;
        var panelTonesHeader = MakeSectionHeader("Panel Tones", ay);
        tab.Controls.Add(panelTonesHeader); advancedOnlyControls.Add(panelTonesHeader);
        ay += 30;
        foreach (var tone in allTones)
        {
            var cb = MakeCheckBox(
                $"{tone.Icon()}  {tone.DisplayName()}",
                App.Settings.EnabledTones.Contains(tone.ApiValue()),
                ay);
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
            tab.Controls.Add(cb); advancedOnlyControls.Add(cb);
            ay += 26;
        }
        int advancedBlockHeight = ay - conditionalY;

        // ── Translation section (always visible, position depends on mode) ──
        // Constructed at y=0 then re-positioned by UpdateModeVisibility so it
        // always sits directly below the visible block — no gaps, no overlap.
        var translationHeader = MakeSectionHeader("Translation", 0);
        var translationLabel = MakeFieldLabel("Target Language", 0);

        // Editable ComboBox: pre-populated with common languages but the user
        // can type any value for niche ones — the backend passes the string
        // straight to the AI, so anything human-readable works.
        var langBox = new WinForms.ComboBox
        {
            Location = new Point(16, 0),
            Size = new Size(448, 30),
            BackColor = CardBg,
            ForeColor = TextPrimary,
            Font = new Font("Segoe UI", 10),
            DropDownStyle = WinForms.ComboBoxStyle.DropDown,
            FlatStyle = WinForms.FlatStyle.Flat,
            AutoCompleteMode = WinForms.AutoCompleteMode.SuggestAppend,
            AutoCompleteSource = WinForms.AutoCompleteSource.ListItems,
        };
        string[] commonLanguages =
        {
            "English", "Vietnamese", "Spanish", "French", "German", "Italian",
            "Portuguese", "Dutch", "Russian", "Japanese", "Korean",
            "Chinese (Simplified)", "Chinese (Traditional)", "Arabic", "Hindi",
            "Thai", "Indonesian", "Turkish", "Polish",
        };
        langBox.Items.AddRange(commonLanguages);
        langBox.Text = App.Settings.TranslateLanguage ?? "";
        langBox.TextChanged += (_, _) =>
        {
            App.Settings.TranslateLanguage = langBox.Text.Trim();
            App.Settings.Save();
        };
        tab.Controls.Add(translationHeader);
        tab.Controls.Add(translationLabel);
        tab.Controls.Add(langBox);

        void UpdateModeVisibility()
        {
            bool isSimple = modeCombo.SelectedIndex == 1;
            foreach (var c in simpleOnlyControls) c.Visible = isSimple;
            foreach (var c in advancedOnlyControls) c.Visible = !isSimple;

            int visibleHeight = isSimple ? simpleBlockHeight : advancedBlockHeight;
            int translationY = conditionalY + visibleHeight + 14;
            translationHeader.Top = translationY;
            translationLabel.Top  = translationY + 30;
            langBox.Top           = translationY + 48;
        }
        UpdateModeVisibility();

        modeCombo.SelectedIndexChanged += (_, _) =>
        {
            App.Settings.AppMode = modeCombo.SelectedIndex == 1 ? AppMode.OneClick : AppMode.Advanced;
            App.Settings.Save();
            UpdateModeVisibility();
        };

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
        PopulateAccountTab(tab);
        return tab;
    }

    /// <summary>
    /// Renders the Account tab body based on current auth state. Called once
    /// at tab construction, then again whenever the user signs in or out so
    /// the UI flips between Sign In and Signed In without requiring the
    /// Settings window to be reopened.
    /// </summary>
    private static void PopulateAccountTab(WinForms.TabPage tab)
    {
        // Tear down existing controls before re-rendering. Dispose explicitly so
        // event handlers don't keep references to disposed boxes/buttons alive.
        var existing = new System.Collections.Generic.List<WinForms.Control>();
        foreach (WinForms.Control c in tab.Controls) existing.Add(c);
        tab.Controls.Clear();
        foreach (var c in existing) c.Dispose();

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
                PopulateAccountTab(tab);
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
                        // Flip the tab to the Signed In view immediately.
                        PopulateAccountTab(tab);
                    }
                    else
                    {
                        setStatus("Login failed.", ErrorRed);
                    }
                }
                catch (Exception ex)
                {
                    setStatus(ex.ToString(), ErrorRed);
                    DRLogger.Error($"Login error: {ex}", DRLogger.Category.AUTH);
                }
                finally
                {
                    // signInBtn may have been disposed if PopulateAccountTab ran.
                    try { signInBtn.Enabled = true; } catch (ObjectDisposedException) { }
                }
            };
            tab.Controls.Add(signInBtn);
        }
    }

    private static WinForms.TabPage BuildAdvancedTab()
    {
        var tab = MakeTab("Advanced");
        int y = 16;

        tab.Controls.Add(MakeSectionHeader("Logs", y));
        y += 30;

        var loggingEnabled = MakeCheckBox("Enable Logging", App.Settings.LoggingEnabled, y);
        loggingEnabled.CheckedChanged += (_, _) =>
        {
            App.Settings.LoggingEnabled = loggingEnabled.Checked;
            App.Settings.Save();
            DRLogger.IsEnabled = loggingEnabled.Checked;
        };
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
        y += 50;

        // Feedback section — opens the bug-report dialog (mirrors macOS).
        tab.Controls.Add(MakeSectionHeader("Feedback", y));
        y += 30;
        tab.Controls.Add(new WinForms.Label
        {
            Text = "Hit a bug? Send us a description (and a screenshot if you have one) and we'll take a look.",
            ForeColor = TextMuted,
            Font = new Font("Segoe UI", 9),
            Location = new Point(16, y),
            Size = new Size(448, 36),
        });
        y += 40;
        var bugBtn = MakeSecondaryButton("Report a Bug…", 16, y, 160);
        bugBtn.Click += (_, _) => Views.ReportBugDialog.Show();
        tab.Controls.Add(bugBtn);

        var featureBtn = MakeSecondaryButton("Suggest a feature…", 188, y, 180);
        featureBtn.Click += (_, _) => Views.SuggestFeatureDialog.Show();
        tab.Controls.Add(featureBtn);

        return tab;
    }
}
