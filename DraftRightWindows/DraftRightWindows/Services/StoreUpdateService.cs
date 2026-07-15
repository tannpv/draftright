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
/// Both calls are no-ops for sideload (.exe) builds — the SignatureKind check
/// in <see cref="IsStoreInstall"/> gates that, so the App.cs factory only
/// constructs this service when running as a Store package.
/// </summary>
public class StoreUpdateService : IUpdateService
{
    private readonly StoreContext _context;
    private readonly string _currentVersion;
    private DateTime _lastCheck = DateTime.MinValue;
    private const int CheckIntervalHours = 24;

    /// <inheritdoc/>
    public UpdateInfo? AvailableUpdate { get; private set; }

    /// <inheritdoc/>
    /// <remarks>For Store installs the Store agent owns staging — we don't
    /// pre-download here. Reported false so the UI shows "Update now" rather
    /// than a misleading "Restart to install" affordance.</remarks>
    public bool UpdateStaged => false;

    /// <inheritdoc/>
    public event Action? AvailableUpdateChanged;

    public StoreUpdateService(string currentVersion)
    {
        _currentVersion = currentVersion;
        _context = StoreContext.GetDefault();
    }

    /// <summary>True iff Package.Current reports a Store-signed identity.
    /// Wrapped in try/catch because non-packaged sideload .exe builds throw
    /// InvalidOperationException on Package.Current access.</summary>
    public static bool IsStoreInstall()
    {
        try
        {
            return Package.Current.SignatureKind == PackageSignatureKind.Store;
        }
        catch
        {
            return false;
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
        if ((DateTime.UtcNow - _lastCheck).TotalHours < CheckIntervalHours)
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
                var v = first.Package.Id.Version;
                var versionString = $"{v.Major}.{v.Minor}.{v.Build}";
                AvailableUpdate = new UpdateInfo
                {
                    Version = versionString,
                    WindowsUrl = "ms-windows-store://pdp/?ProductId=" + (Package.Current.Id.FamilyName),
                    ReleaseNotes = string.Empty, // Store does not expose per-update notes via API.
                    Required = first.Mandatory,
                };
            }

            // Only fire if state actually changed (version differs or null↔value)
            var changed = (previous?.Version ?? "") != (AvailableUpdate?.Version ?? "");
            if (changed)
            {
                DRLogger.Log(
                    AvailableUpdate != null
                        ? $"Store update available: {AvailableUpdate.Version} (mandatory={AvailableUpdate.Required})"
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
            DRLogger.Log($"Requesting Store install of {info.Version}…", DRLogger.Category.APP);
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
            // user can retry from the same button.
        }
        catch (Exception ex)
        {
            DRLogger.Error($"StoreUpdateService.StartInstall failed: {ex.Message}",
                DRLogger.Category.APP);
        }
    }
}
