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
    private System.Threading.Timer? _healthTimer;
    // Owns the tray icon, menu, update badge, and its STA pump thread.
    private TrayIconController? _tray;
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
    // The rewrite UI is a WinForms form (RewritePanelForm), not WinUI XAML: on
    // unpackaged builds without the VS AppX tooling (no real resources.pri), a
    // WinUI panel crashed on first render (STATUS_STOWED_EXCEPTION 0xc000027b —
    // "Cannot find a Resource ... TabViewScrollButtonBackground"). The WinForms
    // form shares the same ViewModel API + dark theme and works on local x64.
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

        // Create UpdateService BEFORE starting the tray. The tray subscribes to
        // its AvailableUpdateChanged event in its ctor, so it must already
        // exist — otherwise the badge / "update available" menu never updates
        // when an update is found.
        // Read the real assembly version (hardcoding made every backend release
        // look newer than the installed build).
        var asmVer = System.Reflection.Assembly.GetExecutingAssembly()
            .GetName().Version?.ToString() ?? "0.0.0";
        // Drop trailing ".0" so the local string compares cleanly against
        // semver-style backend versions ("2.1.1" not "2.1.1.0").
        var currentVersion = asmVer.EndsWith(".0") ? asmVer.Substring(0, asmVer.Length - 2) : asmVer;
        DRLogger.Log($"UpdateService current version: {currentVersion} (assembly {asmVer})",
            DRLogger.Category.APP);
        UpdateService = new UpdateService(currentVersion, Settings.BackendUrl);

        _tray = new TrayIconController(
            UpdateService,
            onOpenSettings: OpenSettings,
            onSignOut: () => { Auth.ClearTokens(); Api.ClearToken(); },
            onQuit: DoQuit);
        _tray.Start();

        // Start health check — immediate first check, then every 30 seconds
        _healthTimer = new System.Threading.Timer(async _ => await PerformHealthCheckAsync(), null, TimeSpan.Zero, TimeSpan.FromSeconds(30));

        // Start update check — 10 seconds after launch.
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
    /// Captures selected text from the foreground app and opens the rewrite panel.
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

                // Advanced mode: auto-run the configured "Default Tone" on open
                // (if set and still enabled), so the panel produces a rewrite
                // without requiring a tone click — mirrors the macOS auto-run
                // behavior. Empty default = manual pick (unchanged).
                var defaultToneApi = Settings.DefaultTone;
                if (!string.IsNullOrEmpty(defaultToneApi)
                    && Settings.EnabledTones.Contains(defaultToneApi)
                    && ToneExtensions.FromApiValue(defaultToneApi) is Tone autoTone)
                {
                    panel.AutoRunTone = autoTone;
                }

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

        _tray?.ShowError("DraftRight - One-Click Rewrite Failed", message);
    }

    private async Task PerformHealthCheckAsync()
    {
        var status = await Api.CheckHealthAsync();
        CurrentStatus = status;

        // Reflect status on the tray (tooltip + status menu header).
        var label = status switch
        {
            BackendStatus.Connected => "Connected",
            BackendStatus.NotLoggedIn => "Not Logged In",
            BackendStatus.WrongServer => "Wrong Server",
            _ => "Offline"
        };
        _tray?.SetStatus(label);

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

        _tray?.Dispose();
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
