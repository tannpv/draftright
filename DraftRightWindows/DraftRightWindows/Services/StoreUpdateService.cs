using System.Diagnostics;
using DraftRightWindows.Helpers;
using Windows.ApplicationModel;
using Windows.Services.Store;

namespace DraftRightWindows.Services;

/// <summary>
/// Update backend for installs that came from Microsoft Store (MSIX).
///
/// Microsoft Store already updates Store-installed apps silently in the
/// background when the user has Store auto-updates enabled (the default).
/// This service surfaces that activity inside DraftRight so the tray icon
/// and Settings panel can:
///   - show a "new version available" badge as soon as the Store agent sees one,
///   - offer a "Update now" button that asks the Store to download/install
///     immediately instead of waiting for its own cadence.
///
/// API used (Windows.Services.Store):
///   - StoreContext.GetAppAndOptionalStorePackageUpdatesAsync()
///   - StoreContext.RequestDownloadAndInstallStorePackageUpdatesAsync()
///
/// Both calls are no-ops for sideload (.exe) builds — <see cref="IsPackaged"/>
/// gates that, so the App.cs factory only constructs this service when running
/// as a packaged (MSIX) app.
///
/// IMPORTANT — owner window: <c>StoreContext</c> was designed for UWP, where
/// the app has an implicit CoreWindow. In a desktop (Win32/WinUI 3) process
/// there is none, so every StoreContext call that shows UI — which includes
/// <c>RequestDownloadAndInstallStorePackageUpdatesAsync</c> — fails with
/// <c>0x80070578 ERROR_INVALID_WINDOW_HANDLE</c> unless the context has first
/// been associated with an HWND via <c>IInitializeWithWindow</c>. That was the
/// cause of "clicking the update link does nothing": the check found the
/// update and the tray/Settings link appeared, but the install call threw
/// immediately and the failure only ever reached the log.
/// </summary>
public class StoreUpdateService : IUpdateService
{
    /// <summary>Store deep link to the Downloads &amp; updates pane. Used as
    /// the escape hatch when the in-app install request fails, so a user is
    /// never left with a button that does nothing.</summary>
    private const string StoreUpdatesUri = "ms-windows-store://downloadsandupdates";

    /// <summary>Store deep link to this app's product page. <c>{0}</c> is the
    /// package family name. The <c>PFN</c> key is required — the <c>ProductId</c>
    /// key expects a Store id (<c>9Nxxxxxxxxxx</c>) and silently fails when
    /// handed a family name.</summary>
    private const string StoreProductPageUriFormat = "ms-windows-store://pdp/?PFN={0}";

    private readonly StoreContext _context;
    private DateTime _lastCheck = DateTime.MinValue;

    /// <inheritdoc/>
    public UpdateInfo? AvailableUpdate { get; private set; }

    /// <inheritdoc/>
    /// <remarks>For Store installs the Store agent owns staging — we don't
    /// pre-download here. Reported false so the UI shows "Update now" rather
    /// than a misleading "Restart to install" affordance.</remarks>
    public bool UpdateStaged => false;

    /// <inheritdoc/>
    public event Action? AvailableUpdateChanged;

    /// <param name="ownerHwnd">Window handle that owns any Store-rendered
    /// dialog. Must be a live HWND from this process — see the class remarks;
    /// without it the install request throws 0x80070578.</param>
    /// <remarks>Deliberately takes no current-version argument: unlike the HTTP
    /// backend, this one never compares versions itself — the Store agent
    /// decides which package updates apply to this install.</remarks>
    public StoreUpdateService(IntPtr ownerHwnd)
    {
        _context = StoreContext.GetDefault();

        if (ownerHwnd == IntPtr.Zero)
        {
            // Not fatal: update *detection* still works, only the install
            // request will fail. Loud so the log says why if it ever happens.
            DRLogger.Error(
                "StoreUpdateService: owner HWND is null — Store install requests will fail with 0x80070578.",
                DRLogger.Category.APP);
            return;
        }

        try
        {
            WinRT.Interop.InitializeWithWindow.Initialize(_context, ownerHwnd);
            DRLogger.Log($"StoreUpdateService: StoreContext bound to hwnd=0x{ownerHwnd.ToInt64():X}",
                DRLogger.Category.APP);
        }
        catch (Exception ex)
        {
            DRLogger.Error($"StoreUpdateService: InitializeWithWindow failed: {ex.Message}",
                DRLogger.Category.APP);
        }
    }

    /// <summary>True iff the app is running as a packaged (MSIX) app of ANY
    /// signature — Store, Developer, or Enterprise. The HTTP `.exe` self-updater
    /// can never replace an MSIX package, so any packaged build must avoid it
    /// (a Store-signed check alone let sideloaded/dev-signed MSIX builds fall
    /// through to the HTTP updater and hang on the "Downloading" window).
    /// Wrapped in try/catch because non-packaged .exe builds throw on
    /// Package.Current access.</summary>
    public static bool IsPackaged()
    {
        try
        {
            _ = Package.Current.Id;
            return true;
        }
        catch
        {
            return false;
        }
    }

    /// <inheritdoc/>
    public async Task CheckIfNeededAsync()
    {
        if ((DateTime.UtcNow - _lastCheck).TotalHours < UpdatePolicy.CheckIntervalHours)
            return;
        await RefreshAvailableUpdateAsync();
    }

    /// <inheritdoc/>
    public Task CheckNowAsync() => RefreshAvailableUpdateAsync().ContinueWith(_ => { });

    /// <inheritdoc/>
    public async Task<UpdateInfo?> RefreshAvailableUpdateAsync()
    {
        try
        {
            _lastCheck = DateTime.UtcNow;
            var updates = await _context.GetAppAndOptionalStorePackageUpdatesAsync();

            var previous = AvailableUpdate;
            if (updates == null || updates.Count == 0)
            {
                AvailableUpdate = null;
            }
            else
            {
                // Take the first applicable update — for a single-package app
                // (DraftRight is one MSIX) there's only ever one entry anyway.
                var first = updates[0];
                AvailableUpdate = new UpdateInfo
                {
                    // Deliberately blank. StorePackageUpdate.Package.Id.Version
                    // reports the version of the package being replaced, NOT the
                    // one being offered, so putting it in the UI told users
                    // "Update <the version you are already running> available".
                    // Observed twice on a real Store install: 2.3.25 advertised
                    // while running 2.3.25, and 2.3.52 advertised while running
                    // 2.3.52 with 2.3.61 the actual pending package. UpdateLabel
                    // degrades to a version-less "Update available" for this.
                    Version = string.Empty,
                    WindowsUrl = string.Format(StoreProductPageUriFormat, Package.Current.Id.FamilyName),
                    ReleaseNotes = string.Empty, // Store does not expose per-update notes via API.
                    Required = first.Mandatory,
                };
                LogStoreReportedVersion(first);
            }

            // Fire on appearance/disappearance only. This backend has no
            // trustworthy version to diff, so comparing version strings (as the
            // HTTP backend does) would compare "" to "" and never fire at all.
            var changed = (previous == null) != (AvailableUpdate == null);
            if (changed)
            {
                DRLogger.Log(
                    AvailableUpdate != null
                        ? $"Store update available (mandatory={AvailableUpdate.Required})"
                        : "Store reports no updates available.",
                    DRLogger.Category.APP);
                AvailableUpdateChanged?.Invoke();
            }

            return AvailableUpdate;
        }
        catch (Exception ex)
        {
            DRLogger.Error($"StoreUpdateService.RefreshAvailableUpdateAsync failed: {ex.Message}",
                DRLogger.Category.APP);
            return null;
        }
    }

    /// <summary>Records what the Store actually reported, for diagnosis only —
    /// never for display. Keeping it in the log is what made the "advertised
    /// version equals installed version" pattern visible in the first place, so
    /// a future reader can re-check the assumption against a live install
    /// instead of taking this class's word for it.</summary>
    private static void LogStoreReportedVersion(StorePackageUpdate update)
    {
        var v = update.Package.Id.Version;
        DRLogger.Log(
            $"Store-reported package version (untrusted, not shown): {v.Major}.{v.Minor}.{v.Build}.{v.Revision}",
            DRLogger.Category.APP);
    }

    /// <inheritdoc/>
    /// <remarks>The Microsoft Store API does not expose per-update release
    /// notes — Store-installed users see notes in the Store listing page, not
    /// in-app. Returning null causes the post-update "What's New" notice to
    /// be skipped (rather than showing an empty popup).</remarks>
    public Task<string?> GetReleaseNotesForVersionAsync(string version) =>
        Task.FromResult<string?>(null);

    /// <inheritdoc/>
    public async void StartInstall(UpdateInfo info)
    {
        try
        {
            DRLogger.Log("Requesting Store install of the pending update…", DRLogger.Category.APP);
            var updates = await _context.GetAppAndOptionalStorePackageUpdatesAsync();
            if (updates == null || updates.Count == 0)
            {
                DRLogger.Warn("StartInstall called but Store reports no updates — refreshing.",
                    DRLogger.Category.APP);
                AvailableUpdate = null;
                AvailableUpdateChanged?.Invoke();
                return;
            }

            var result = await _context.RequestDownloadAndInstallStorePackageUpdatesAsync(updates);
            DRLogger.Log($"Store install result: {result.OverallState}", DRLogger.Category.APP);

            // Successful install relaunches the app via Store agent — nothing
            // else to do here. On failure, leave AvailableUpdate set so the
            // user can retry from the same button, and hand them off to the
            // Store UI so the click still leads somewhere.
            if (IsFailure(result.OverallState))
                OpenStoreFallback(info);
        }
        catch (Exception ex)
        {
            DRLogger.Error($"StoreUpdateService.StartInstall failed: {ex.Message}",
                DRLogger.Category.APP);
            OpenStoreFallback(info);
        }
    }

    /// <summary>True for the terminal states that mean the update did not and
    /// will not happen, so the user needs another route. Deliberately excludes
    /// <c>Canceled</c> — the user declining the Store's own dialog is an answer,
    /// not a failure, and bouncing them into the Store would override it — and
    /// the in-progress states (<c>Pending</c>/<c>Downloading</c>/<c>Deploying</c>),
    /// where the Store agent is still working and will finish on its own.</summary>
    private static bool IsFailure(StorePackageUpdateState state) => state is
        StorePackageUpdateState.OtherError or
        StorePackageUpdateState.ErrorLowBattery or
        StorePackageUpdateState.ErrorWiFiRecommended or
        StorePackageUpdateState.ErrorWiFiRequired;

    /// <summary>Last resort when the in-app install request doesn't complete:
    /// open the Store so the user can finish the update by hand. Tries the
    /// app's product page first and falls back to Downloads &amp; updates.
    /// Silent failure here is acceptable — it is already the error path.</summary>
    private static void OpenStoreFallback(UpdateInfo info)
    {
        var uri = !string.IsNullOrEmpty(info.WindowsUrl) ? info.WindowsUrl : StoreUpdatesUri;
        try
        {
            DRLogger.Log($"Store install did not complete — opening {uri}", DRLogger.Category.APP);
            Process.Start(new ProcessStartInfo(uri) { UseShellExecute = true });
        }
        catch (Exception ex)
        {
            DRLogger.Warn($"StoreUpdateService: could not open {uri}: {ex.Message}",
                DRLogger.Category.APP);
        }
    }
}
