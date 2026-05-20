using System;
using System.Diagnostics;
using System.IO;
using System.Xml;

namespace DraftRightWindows.Services;

/// <summary>
/// Windows port of the macOS launchd KeepAlive agent. Registers a per-user
/// Windows Scheduled Task that:
///   • launches DraftRight at logon (RunAtLogon), and
///   • restarts the process if it exits unexpectedly (RestartOnFailure).
///
/// Why a Scheduled Task instead of <c>HKCU\...\Run</c>:
/// the registry "Run" key only fires at logon — nothing brings the app
/// back if it dies mid-session. Task Scheduler's RestartOnFailure trigger
/// covers crash/OS-kill cases the way macOS launchd's KeepAlive does.
///
/// We shell out to <c>schtasks.exe</c> with an XML task definition so we
/// don't pull in a NuGet dependency (Microsoft.Win32.TaskScheduler).
/// </summary>
public static class KeepAliveAgent
{
    public const string TaskName = "DraftRight Keep Alive";

    /// <summary>Path to the currently-running app executable.</summary>
    public static string? ExecutablePath =>
        Process.GetCurrentProcess().MainModule?.FileName;

    /// <summary>Path of the installed task XML the task was created from.
    /// Read it back to compare against the running ExecutablePath so the
    /// task self-heals when the app is moved.</summary>
    public static string TaskXmlBackupPath
    {
        get
        {
            var localAppData = Environment.GetFolderPath(
                Environment.SpecialFolder.LocalApplicationData);
            return Path.Combine(localAppData, "DraftRight", "keepalive.xml");
        }
    }

    /// <summary>True if a task with our name currently exists.</summary>
    public static bool IsInstalled => RunSchtasks(
        "/Query /TN \"" + TaskName + "\"", out _, out _);

    /// <summary>
    /// True if the existing task's Action Command matches the currently
    /// running executable path. False means the app was moved since the
    /// task was last written — caller should reinstall.
    /// </summary>
    public static bool IsPathFresh
    {
        get
        {
            if (!IsInstalled) return false;
            if (!File.Exists(TaskXmlBackupPath)) return false;
            var current = ExecutablePath;
            if (string.IsNullOrEmpty(current)) return false;
            try
            {
                var doc = new XmlDocument();
                doc.Load(TaskXmlBackupPath);
                var ns = new XmlNamespaceManager(doc.NameTable);
                ns.AddNamespace(
                    "t", "http://schemas.microsoft.com/windows/2004/02/mit/task");
                var node = doc.SelectSingleNode(
                    "//t:Actions/t:Exec/t:Command", ns);
                return string.Equals(
                    node?.InnerText, current, StringComparison.OrdinalIgnoreCase);
            }
            catch
            {
                return false;
            }
        }
    }

    /// <summary>Register (or refresh) the scheduled task.</summary>
    /// <param name="runAtLogon">Whether to launch the app automatically when
    /// the user logs in. The crash-respawn behavior is always enabled when
    /// the task exists at all.</param>
    public static bool Install(bool runAtLogon)
    {
        var exe = ExecutablePath;
        if (string.IsNullOrEmpty(exe))
        {
            DRLogger.Log("KeepAliveAgent: no executable path — aborting install",
                DRLogger.Category.APP);
            return false;
        }

        try
        {
            var xml = BuildTaskXml(exe, runAtLogon);
            Directory.CreateDirectory(
                Path.GetDirectoryName(TaskXmlBackupPath)!);
            File.WriteAllText(TaskXmlBackupPath, xml);

            // /F overwrites if the task already exists. Per-user
            // (Interactive, LeastPrivilege) so no UAC prompt is needed.
            var args = "/Create /F /TN \"" + TaskName + "\" /XML \"" +
                       TaskXmlBackupPath + "\"";
            if (!RunSchtasks(args, out var stdout, out var stderr))
            {
                DRLogger.Log("KeepAliveAgent: schtasks /Create failed — " +
                             stderr, DRLogger.Category.APP);
                return false;
            }
            DRLogger.Log("KeepAliveAgent: installed (runAtLogon=" +
                         runAtLogon + ")", DRLogger.Category.APP);
            return true;
        }
        catch (Exception ex)
        {
            DRLogger.Log("KeepAliveAgent: install failed — " + ex.Message,
                DRLogger.Category.APP);
            return false;
        }
    }

    /// <summary>Delete the scheduled task entirely.</summary>
    public static bool Uninstall()
    {
        if (!IsInstalled) return true;
        var args = "/Delete /F /TN \"" + TaskName + "\"";
        if (!RunSchtasks(args, out _, out var stderr))
        {
            DRLogger.Log("KeepAliveAgent: schtasks /Delete failed — " + stderr,
                DRLogger.Category.APP);
            return false;
        }
        try
        {
            if (File.Exists(TaskXmlBackupPath))
                File.Delete(TaskXmlBackupPath);
        }
        catch { /* best-effort */ }
        DRLogger.Log("KeepAliveAgent: uninstalled", DRLogger.Category.APP);
        return true;
    }

    /// <summary>Sync on-disk task to desired state. Idempotent — safe on
    /// every launch.</summary>
    public static void Reconcile(bool desiredRunAtLogon)
    {
        if (desiredRunAtLogon)
        {
            if (!IsInstalled || !IsPathFresh)
            {
                Install(runAtLogon: true);
            }
        }
        else
        {
            if (IsInstalled) Uninstall();
        }
    }

    /// <summary>
    /// Build the Task Scheduler XML. RestartOnFailure with Interval=PT1M and
    /// Count=10 approximates the macOS ThrottleInterval / KeepAlive guard.
    /// Task Scheduler rejects any RestartOnFailure interval below one minute
    /// ("The task XML contains a value which is incorrectly formatted or out
    /// of range. ...Interval:PT10S") — PT1M is the smallest it accepts, so we
    /// can't mirror launchd's 10s exactly. Hidden=true keeps the task out of
    /// casual Task Scheduler UI clutter.
    /// </summary>
    private static string BuildTaskXml(string exePath, bool runAtLogon)
    {
        var user = Environment.UserDomainName + "\\" + Environment.UserName;
        var enabledLogon = runAtLogon ? "true" : "false";
        return
            "<?xml version=\"1.0\" encoding=\"UTF-16\"?>\n" +
            "<Task version=\"1.4\" xmlns=\"http://schemas.microsoft.com/windows/2004/02/mit/task\">\n" +
            "  <RegistrationInfo>\n" +
            "    <Description>Auto-launch DraftRight at logon and respawn on crash. Managed by the app — do not edit by hand.</Description>\n" +
            "  </RegistrationInfo>\n" +
            "  <Triggers>\n" +
            "    <LogonTrigger>\n" +
            "      <Enabled>" + enabledLogon + "</Enabled>\n" +
            "      <UserId>" + System.Security.SecurityElement.Escape(user) + "</UserId>\n" +
            "    </LogonTrigger>\n" +
            "  </Triggers>\n" +
            "  <Principals>\n" +
            "    <Principal id=\"Author\">\n" +
            "      <UserId>" + System.Security.SecurityElement.Escape(user) + "</UserId>\n" +
            "      <LogonType>InteractiveToken</LogonType>\n" +
            "      <RunLevel>LeastPrivilege</RunLevel>\n" +
            "    </Principal>\n" +
            "  </Principals>\n" +
            "  <Settings>\n" +
            "    <RestartOnFailure>\n" +
            "      <Interval>PT1M</Interval>\n" +
            "      <Count>10</Count>\n" +
            "    </RestartOnFailure>\n" +
            "    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>\n" +
            "    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>\n" +
            "    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>\n" +
            "    <AllowHardTerminate>false</AllowHardTerminate>\n" +
            "    <StartWhenAvailable>true</StartWhenAvailable>\n" +
            "    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>\n" +
            "    <IdleSettings>\n" +
            "      <StopOnIdleEnd>false</StopOnIdleEnd>\n" +
            "      <RestartOnIdle>false</RestartOnIdle>\n" +
            "    </IdleSettings>\n" +
            "    <AllowStartOnDemand>true</AllowStartOnDemand>\n" +
            "    <Enabled>true</Enabled>\n" +
            "    <Hidden>true</Hidden>\n" +
            "    <RunOnlyIfIdle>false</RunOnlyIfIdle>\n" +
            "    <WakeToRun>false</WakeToRun>\n" +
            "    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>\n" +
            "    <Priority>7</Priority>\n" +
            "  </Settings>\n" +
            "  <Actions Context=\"Author\">\n" +
            "    <Exec>\n" +
            "      <Command>" + System.Security.SecurityElement.Escape(exePath) + "</Command>\n" +
            "    </Exec>\n" +
            "  </Actions>\n" +
            "</Task>\n";
    }

    private static bool RunSchtasks(
        string arguments, out string stdout, out string stderr)
    {
        stdout = string.Empty;
        stderr = string.Empty;
        try
        {
            using var proc = new Process
            {
                StartInfo = new ProcessStartInfo
                {
                    FileName = "schtasks.exe",
                    Arguments = arguments,
                    UseShellExecute = false,
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    CreateNoWindow = true,
                },
            };
            proc.Start();
            stdout = proc.StandardOutput.ReadToEnd();
            stderr = proc.StandardError.ReadToEnd();
            proc.WaitForExit();
            return proc.ExitCode == 0;
        }
        catch (Exception ex)
        {
            stderr = ex.Message;
            return false;
        }
    }
}
