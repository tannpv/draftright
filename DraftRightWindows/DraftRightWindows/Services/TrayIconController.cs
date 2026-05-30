using System;
using System.Drawing;
using System.Threading;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows.Services;

/// <summary>
/// Owns the system-tray icon, its context menu, the "update available" badge,
/// and the dedicated STA message-pump thread they live on. Extracted from App
/// so the tray plumbing is one cohesive unit.
///
/// Threading notes (load-bearing — these were the source of two badge bugs):
///  • Runs on its own STA thread with a no-arg <c>Application.Run()</c> pump.
///  • A hidden, never-shown <see cref="WinForms.Form"/> (handle forced at
///    startup) is the marshaling target for cross-thread updates —
///    <c>ContextMenuStrip.BeginInvoke</c> throws until the menu is first opened.
///  • Construct + <see cref="Start"/> AFTER <see cref="UpdateService"/> exists
///    so the <c>AvailableUpdateChanged</c> subscription is always wired.
/// </summary>
internal sealed class TrayIconController : IDisposable
{
    private readonly IUpdateService _updateService;
    private readonly Action _onOpenSettings;
    private readonly Action _onSignOut;
    private readonly Action _onQuit;

    private Thread? _thread;
    private WinForms.NotifyIcon? _trayIcon;
    // Hidden marshaling target — see class remarks.
    private WinForms.Form? _trayPump;
    private WinForms.ToolStripMenuItem? _statusMenuItem;
    private WinForms.ToolStripMenuItem? _updateMenuItem;
    // Base icon + a cached copy with the "update ready" badge dot; swapped on
    // _trayIcon as the update state changes.
    private Icon? _baseTrayIcon;
    private Icon? _badgeTrayIcon;
    // Tracks the last badge state so the balloon fires once on the rising edge.
    private bool _updateBadgeShown;

    public TrayIconController(
        IUpdateService updateService,
        Action onOpenSettings,
        Action onSignOut,
        Action onQuit)
    {
        _updateService = updateService;
        _onOpenSettings = onOpenSettings;
        _onSignOut = onSignOut;
        _onQuit = onQuit;
    }

    /// <summary>Starts the tray on its own STA thread. Returns immediately.</summary>
    public void Start()
    {
        _thread = new Thread(RunLoop);
        _thread.SetApartmentState(ApartmentState.STA);
        _thread.IsBackground = true;
        _thread.Start();
    }

    private void RunLoop()
    {
        WinForms.Application.EnableVisualStyles();

        // Marshaling target with a guaranteed handle (forced below). Never
        // shown, so it's invisible; Application.Run() still pumps its messages.
        _trayPump = new WinForms.Form { ShowInTaskbar = false };
        _ = _trayPump.Handle; // force handle creation on the tray thread

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
            var u = _updateService.AvailableUpdate;
            if (u != null) _updateService.StartInstall(u);
        };
        menu.Items.Add(_updateMenuItem);
        menu.Items.Add(new WinForms.ToolStripSeparator());
        menu.Items.Add("Settings", null, (_, _) => _onOpenSettings());
        menu.Items.Add(new WinForms.ToolStripSeparator());
        menu.Items.Add("Sign Out", null, (_, _) => _onSignOut());
        menu.Items.Add("Quit", null, (_, _) => _onQuit());

        _trayIcon.ContextMenuStrip = menu;
        _trayIcon.DoubleClick += (_, _) => _onOpenSettings();
        _trayIcon.Visible = true;

        // Reflect "update available" in the tray menu — both now (in case the
        // 10s-after-launch check already finished) and whenever it changes.
        _updateService.AvailableUpdateChanged += () =>
        {
            DRLogger.Log($"AvailableUpdateChanged fired (pumpHandle={_trayPump?.IsHandleCreated})", DRLogger.Category.APP);
            try { _trayPump?.BeginInvoke(new Action(RefreshUpdateMenuItem)); }
            catch (Exception ex) { DRLogger.Error($"AvailableUpdateChanged marshal failed: {ex.GetType().Name}: {ex.Message}", DRLogger.Category.APP); }
        };
        RefreshUpdateMenuItem();

        WinForms.Application.Run();
    }

    // Show/hide + relabel the tray "update available" item from the current
    // UpdateService.AvailableUpdate. Must run on the tray thread.
    private void RefreshUpdateMenuItem()
    {
        if (_updateMenuItem == null) return;
        var u = _updateService.AvailableUpdate;
        var staged = u != null && _updateService.UpdateStaged;
        DRLogger.Log($"RefreshUpdateMenuItem: ran (u={u?.Version ?? "(null)"} staged={staged})", DRLogger.Category.APP);
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
            DRLogger.Log($"UpdateTrayBadge: hasUpdate={hasUpdate} badgeIconNull={_badgeTrayIcon == null} visible={_trayIcon.Visible} — icon set", DRLogger.Category.APP);

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

    /// <summary>Update the status line (tray tooltip + the disabled menu
    /// header). Set directly (best-effort, like the original) — NotifyIcon.Text
    /// has no handle-thread affinity and a stale label is harmless.</summary>
    public void SetStatus(string label)
    {
        if (_trayIcon == null) return;
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

    /// <summary>Show an error balloon from the tray icon (best-effort).</summary>
    public void ShowError(string title, string message)
    {
        try
        {
            if (_trayIcon != null)
            {
                _trayIcon.BalloonTipTitle = title;
                _trayIcon.BalloonTipText = message;
                _trayIcon.BalloonTipIcon = WinForms.ToolTipIcon.Error;
                _trayIcon.ShowBalloonTip(4000);
            }
        }
        catch
        {
            // Best-effort: tray icon may be disposed or BalloonTip unavailable.
        }
    }

    public void Dispose()
    {
        if (_trayIcon != null)
        {
            _trayIcon.Visible = false;
            _trayIcon.Dispose();
            _trayIcon = null;
        }
        _trayPump?.Dispose();
        _trayPump = null;
        _badgeTrayIcon?.Dispose();
        _badgeTrayIcon = null;
        _baseTrayIcon?.Dispose();
        _baseTrayIcon = null;
    }
}
