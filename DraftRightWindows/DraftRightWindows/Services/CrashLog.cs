using System;
using System.Windows.Threading;

namespace DraftRightWindows.Services;

/// <summary>
/// One place for crash capture. The global handlers (WinUI app, AppDomain,
/// TaskScheduler) in App.cs and the per-UI-thread WPF dispatcher handler all
/// funnel through here, so a crash is always logged, written to the on-disk
/// crash file, and reported to the backend /errors endpoint the same way
/// (RULE #1: one crash path, not one per handler).
///
/// The WPF windows (rewrite panel, settings, dialogs) each run on their own STA
/// thread with Dispatcher.Run(). An exception there raises
/// Dispatcher.UnhandledException, which the WinUI Application-level handler does
/// NOT see — without <see cref="InstallForCurrentDispatcher"/> such an exception
/// takes the whole app down. Installing it turns those into logged-and-reported
/// non-fatal events: the failing window may close, but the process survives.
/// </summary>
public static class CrashLog
{
    /// <summary>Append the exception to the Desktop crash file. Best-effort.</summary>
    public static void WriteCrashFile(string source, Exception? ex)
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

    /// <summary>Log + crash-file + report an exception through the one path.</summary>
    public static void Capture(string source, Exception? ex, string severity = "error")
    {
        DRLogger.Error($"{source}: {ex}", DRLogger.Category.APP);
        WriteCrashFile(source, ex);
        if (ex != null) ErrorReporter.Report(ex, source: source, severity: severity);
    }

    /// <summary>
    /// Installs a Dispatcher.UnhandledException handler on the CURRENT thread's
    /// dispatcher. Call this at the top of every WPF STA thread (each detached
    /// window) before Dispatcher.Run(). Marks the exception handled so the
    /// failing window can unwind without crashing the whole process.
    /// </summary>
    public static void InstallForCurrentDispatcher(string source)
    {
        Dispatcher.CurrentDispatcher.UnhandledException += (_, e) =>
        {
            Capture($"dispatcher:{source}", e.Exception);
            e.Handled = true;
        };
    }
}
