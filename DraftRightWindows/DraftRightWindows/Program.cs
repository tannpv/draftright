using Microsoft.UI.Dispatching;
using Microsoft.UI.Xaml;
using WinRT;

namespace DraftRightWindows;

public static class Program
{
    // Per-user single-instance guard. Local\ prefix keeps the mutex scoped to
    // the current login session so different Windows users on the same machine
    // can each run their own DraftRight (verified by SINGLE-INST-006). The
    // GUID is stable across builds so an older version + a newer version don't
    // both launch side-by-side.
    private const string SingleInstanceMutexName =
        @"Local\DraftRight-SingleInstance-{4e2f8e10-1bcd-4c0a-9c2c-2b1b9c0d3a4f}";

    [STAThread]
    static void Main(string[] args)
    {
        // initiallyOwned=false so we don't have to release manually — the
        // kernel reclaims the mutex when this process exits (including on
        // Task Manager → End Task), letting the next launch acquire cleanly.
        var mutex = new Mutex(initiallyOwned: false, SingleInstanceMutexName, out _);
        bool acquired = false;
        try
        {
            acquired = mutex.WaitOne(TimeSpan.Zero, exitContext: false);
        }
        catch (AbandonedMutexException)
        {
            // Previous owner died without releasing — we now own it.
            acquired = true;
        }

        if (!acquired)
        {
            // Another DraftRight is already running. Exit silently — the
            // existing instance's tray icon is the user's entry point.
            return;
        }

        try
        {
            ComWrappersSupport.InitializeComWrappers();
            Application.Start(p =>
            {
                var context = new DispatcherQueueSynchronizationContext(DispatcherQueue.GetForCurrentThread());
                SynchronizationContext.SetSynchronizationContext(context);
                _ = new App();
            });
        }
        finally
        {
            mutex.ReleaseMutex();
            mutex.Dispose();
        }
    }
}
