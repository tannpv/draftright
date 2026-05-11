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
    /// Master switch — when false, Log() short-circuits without writing.
    /// Settings.LoggingEnabled is mirrored here at startup and on toggle so
    /// hot paths don't have to reach into SettingsService.
    /// </summary>
    public static bool IsEnabled { get; set; } = true;

    static DRLogger()
    {
        Directory.CreateDirectory(LogDir);
    }

    public static void Log(string message, Category category = Category.APP)
    {
        if (!IsEnabled) return;

        var line = $"[{DateTime.Now:yyyy-MM-dd HH:mm:ss.fff}] [{category}] {message}";
        lock (Lock)
        {
            File.AppendAllText(LogFile, line + Environment.NewLine);
        }
#if DEBUG
        System.Diagnostics.Debug.WriteLine(line);
#endif
    }

    public static string LogFilePath => LogFile;
}
