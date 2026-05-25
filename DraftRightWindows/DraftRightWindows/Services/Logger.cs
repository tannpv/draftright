using System;
using System.IO;

namespace DraftRightWindows.Services;

public static class DRLogger
{
    private static readonly string LogDir = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "DraftRight", "Logs");

    private static readonly string LogFile = Path.Combine(LogDir, "draftright.log");
    private static readonly object Lock = new();

    public enum Category { APP, AUTH, API, PANEL, SETTINGS, HOTKEY }

    /// <summary>
    /// Log severity. Lines are tagged <c>[LEVEL]</c> so failures are skimmable
    /// and greppable (<c>Select-String "\[ERROR\]"</c>) instead of buried in
    /// same-level category chatter. <see cref="Level.WARN"/> = something
    /// degraded but handled; <see cref="Level.ERROR"/> = an operation failed.
    /// </summary>
    public enum Level { INFO, WARN, ERROR, OFF }

    /// <summary>
    /// Master switch — when false, Log() short-circuits without writing.
    /// Settings.LoggingEnabled is mirrored here at startup and on toggle so
    /// hot paths don't have to reach into SettingsService.
    /// </summary>
    public static bool IsEnabled { get; set; } = true;

    /// <summary>
    /// Minimum severity to write, set remotely by the admin portal and pushed
    /// to clients via <c>/health</c>'s <c>client_log_level</c>. The master
    /// verbosity control: <see cref="Level.INFO"/> logs everything (default),
    /// and <see cref="Level.OFF"/> is the absolute kill-switch (silences even
    /// errors — for privacy/compliance). <see cref="Level.OFF"/> is only ever
    /// used as this threshold, never passed as a line's level.
    /// </summary>
    public static Level MinLevel { get; set; } = Level.INFO;

    static DRLogger()
    {
        Directory.CreateDirectory(LogDir);
    }

    public static void Log(string message, Category category = Category.APP, Level level = Level.INFO)
    {
        // Remote admin threshold (master): drop anything below it; OFF drops all.
        if (level < MinLevel) return;
        // WARN/ERROR are always recorded, even when the user has logging
        // toggled off — a bug report shouldn't be blank for the lines that
        // matter most. Only routine INFO chatter honors the off-switch.
        if (!IsEnabled && level == Level.INFO) return;

        // Pad the level so the [CATEGORY] column stays aligned across lines.
        var line = $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss.fff}] [{level,-5}] [{category}] {message}";
        lock (Lock)
        {
            File.AppendAllText(LogFile, line + Environment.NewLine);
        }
#if DEBUG
        System.Diagnostics.Debug.WriteLine(line);
#endif
    }

    /// <summary>Log a handled-but-degraded condition at <see cref="Level.WARN"/>.</summary>
    public static void Warn(string message, Category category = Category.APP) =>
        Log(message, category, Level.WARN);

    /// <summary>Log a failed operation at <see cref="Level.ERROR"/>.</summary>
    public static void Error(string message, Category category = Category.APP) =>
        Log(message, category, Level.ERROR);

    /// <summary>
    /// Applies the backend's <c>client_log_level</c> ("off" | "errors" |
    /// "warnings" | "info") as the <see cref="MinLevel"/> threshold. Unknown or
    /// empty values fall back to full logging. No-op (and silent) when the
    /// level is unchanged, so the ~30s health poll doesn't spam the log; only a
    /// genuine change is announced. Safe to call from any thread.
    /// </summary>
    public static void SetMinLevelFromServer(string? value)
    {
        var newLevel = (value?.Trim().ToLowerInvariant()) switch
        {
            "off" => Level.OFF,
            "errors" or "error" => Level.ERROR,
            "warnings" or "warning" or "warn" => Level.WARN,
            "info" or "" or null => Level.INFO,
            _ => Level.INFO,
        };
        if (newLevel == MinLevel) return;

        var old = MinLevel;
        // Announce the change so on/off transitions are traceable. When
        // narrowing (e.g. → OFF) record it under the OLD threshold first so the
        // announcement itself isn't dropped; when widening, set then record.
        if (newLevel > old)
        {
            Log($"Client log level changed: {old} -> {newLevel} (server '{value}')", Category.APP, Level.WARN);
            MinLevel = newLevel;
        }
        else
        {
            MinLevel = newLevel;
            Log($"Client log level changed: {old} -> {newLevel} (server '{value}')", Category.APP, Level.WARN);
        }
    }

    public static string LogFilePath => LogFile;
}
