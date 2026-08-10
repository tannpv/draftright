using System;
using System.Threading;
using System.Windows;
using System.Windows.Threading;

namespace DraftRightWindows.Services;

/// <summary>
/// Runs a detached WPF window on its own STA thread with a WPF dispatcher — the
/// pattern every non-main-thread window in this app shares (rewrite panel,
/// settings, feedback dialogs, What's New). Installs the per-thread crash
/// handler (<see cref="CrashLog.InstallForCurrentDispatcher"/>), shows the
/// window, and shuts the dispatcher down when it closes so the thread exits.
///
/// RULE #1: one copy of the "STA thread + crash handler + Dispatcher.Run"
/// boilerplate. It used to be duplicated per launcher.
/// </summary>
internal static class StaWindowHost
{
    /// <param name="crashSource">Label for crash reports from this window's thread.</param>
    /// <param name="factory">Creates the window on the STA thread. Do any
    /// window-specific wiring (event handlers, singleton tracking) here; the host
    /// adds its own Closed→shutdown handler on top.</param>
    public static void Run(string crashSource, Func<Window> factory)
    {
        var thread = new Thread(() =>
        {
            try
            {
                CrashLog.InstallForCurrentDispatcher(crashSource);
                var win = factory();
                win.Closed += (_, _) => Dispatcher.CurrentDispatcher.InvokeShutdown();
                win.Show();
                win.Activate();
                Dispatcher.Run();
            }
            catch (Exception ex)
            {
                DRLogger.Error($"StaWindowHost[{crashSource}]: {ex.GetType().Name}: {ex.Message}", DRLogger.Category.APP);
            }
        });
        thread.SetApartmentState(ApartmentState.STA);
        thread.IsBackground = true;
        thread.Start();
    }
}
