using System;
using Microsoft.Win32;

namespace DraftRightWindows.Services;

/// <summary>
/// Manages DraftRight's "run when Windows starts" registration under
/// HKCU\Software\Microsoft\Windows\CurrentVersion\Run. Per-user — no admin
/// needed. This is the same key the Inno Setup installer's optional
/// "autostart" task writes; the app keeps it in sync with the
/// "Launch at Login" setting so the toggle actually does something and the
/// entry always points at the current exe (e.g. after an in-place update).
/// </summary>
public static class StartupRegistration
{
    private const string RunKeyPath = @"Software\Microsoft\Windows\CurrentVersion\Run";
    private const string ValueName = "DraftRight";

    /// <summary>Adds or removes the Run entry so it matches <paramref name="enabled"/>.</summary>
    public static void SetEnabled(bool enabled)
    {
        try
        {
            using var key = Registry.CurrentUser.OpenSubKey(RunKeyPath, writable: true)
                ?? Registry.CurrentUser.CreateSubKey(RunKeyPath, writable: true);
            if (key == null) return;

            if (enabled)
            {
                var exe = Environment.ProcessPath
                    ?? System.Diagnostics.Process.GetCurrentProcess().MainModule?.FileName;
                if (string.IsNullOrEmpty(exe)) return;
                key.SetValue(ValueName, $"\"{exe}\"", RegistryValueKind.String);
            }
            else
            {
                key.DeleteValue(ValueName, throwOnMissingValue: false);
            }
        }
        catch (Exception ex)
        {
            DRLogger.Log($"StartupRegistration.SetEnabled({enabled}) failed: {ex.Message}",
                DRLogger.Category.SETTINGS);
        }
    }

    /// <summary>True if the Run entry currently exists.</summary>
    public static bool IsEnabled()
    {
        try
        {
            using var key = Registry.CurrentUser.OpenSubKey(RunKeyPath, writable: false);
            return key?.GetValue(ValueName) != null;
        }
        catch
        {
            return false;
        }
    }
}
