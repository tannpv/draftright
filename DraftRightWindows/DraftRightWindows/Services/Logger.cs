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
    public enum Level { INFO, WARN, ERROR }

    /// <summary>
    /// Master switch — when false, Log() short-circuits without writing.
    /// Settings.LoggingEnabled is mirrored here at startup and on toggle so
    /// hot paths don't have to reach into SettingsService.
    /// </summary>
    public static bool IsEnabled { get; set; } = true;

    static DRLogger()
    {
        Directory.CreateDirectory(LogDir);
    }

    public static void Log(string message, Category category = Category.APP, Level level = Level.INFO)
    {
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

    public static string LogFilePath => LogFile;
}
