namespace DraftRightWindows.Services;

/// <summary>
/// Common contract shared by the two update backends:
///   - <see cref="UpdateService"/>: HTTP polling against draftright.info/updates
///     for sideload (.exe installer) builds.
///   - <see cref="StoreUpdateService"/>: <c>Windows.Services.Store</c> API for
///     MSIX builds installed via Microsoft Store.
///
/// The rest of the app (tray icon, Settings UI, taskbar badge) talks to this
/// interface, so the choice of backend is invisible at the call sites. The
/// concrete implementation is picked once at startup in <c>App.cs</c> based on
/// <c>Package.Current.SignatureKind</c>.
/// </summary>
public interface IUpdateService
{
    /// <summary>Newest applicable release, or null when up to date.</summary>
    UpdateInfo? AvailableUpdate { get; }

    /// <summary>True when the installer for <see cref="AvailableUpdate"/> is
    /// already downloaded locally (or, for the Store, queued by the Store
    /// agent), so "install" is instant.</summary>
    bool UpdateStaged { get; }

    /// <summary>Fires whenever <see cref="AvailableUpdate"/> or
    /// <see cref="UpdateStaged"/> change so the UI can re-render badges.</summary>
    event Action? AvailableUpdateChanged;

    /// <summary>Throttled background check (no-op if checked recently).</summary>
    Task CheckIfNeededAsync();

    /// <summary>User-initiated "Check now" — bypasses the throttle.</summary>
    Task CheckNowAsync();

    /// <summary>Force a check and return the result.</summary>
    Task<UpdateInfo?> RefreshAvailableUpdateAsync();

    /// <summary>Begin the install flow for the given update. For HTTP this
    /// runs the staged installer; for the Store it asks the Store agent to
    /// download + install the package update.</summary>
    void StartInstall(UpdateInfo info);

    /// <summary>Fetch the user-facing "What's New" notes for the given version.
    /// HTTP backend hits /updates/notes; Store backend returns null because the
    /// Store API does not expose per-update notes.</summary>
    Task<string?> GetReleaseNotesForVersionAsync(string version);
}
