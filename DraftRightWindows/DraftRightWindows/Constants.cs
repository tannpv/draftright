namespace DraftRightWindows;

/// <summary>
/// App-wide compile-time constants. Edit values here — every consumer
/// (SettingsService default, ErrorReporter fallback, App.cs wiring) reads
/// from this single source.
/// </summary>
public static class Constants
{
    /// <summary>
    /// Production backend URL. Used as the SettingsService default and as
    /// ErrorReporter's hardcoded fallback for crashes that fire before the
    /// user's Settings.BackendUrl is loaded.
    /// </summary>
    public const string DefaultBackendUrl = "https://api.draftright.info";
}
