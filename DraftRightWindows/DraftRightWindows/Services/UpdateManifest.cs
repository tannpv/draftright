// Pure update-manifest model + selection logic — deliberately free of WinUI,
// WinForms, and every other Windows-only dependency.
//
// Why this file is separate from UpdateService.cs (issue #80):
//   The test project used to reference the WinUI app assembly, whose
//   CsWinRT/WinUI module initializer bootstraps the WinRT runtime the moment
//   the assembly loads. On a headless CI runner that hangs during xUnit
//   discovery — no tests run at all, and `--filter` doesn't help because the
//   whole assembly loads before filtering. So the app's unit tests never ran
//   in CI and regressions could only be caught on a dev Windows box.
//
//   Everything here compiles under plain `net8.0`, so DraftRightWindows.PureTests
//   can `<Compile Include>` this file directly and run anywhere — including
//   ubuntu-latest — without ever loading the WinUI assembly.
//
// Keep it that way: no `using System.Windows.Forms`, no Microsoft.UI, no
// P/Invoke into user32. Anything needing those belongs in UpdateService.
using System;
using System.Collections.Generic;
using System.Linq;
using System.Text.Json.Serialization;

namespace DraftRightWindows.Services;

public class UpdateInfo
{
    [JsonPropertyName("version")]
    public string Version { get; set; } = "";

    [JsonPropertyName("mac_url")]
    public string MacUrl { get; set; } = "";

    [JsonPropertyName("windows_url")]
    public string WindowsUrl { get; set; } = "";

    [JsonPropertyName("linux_url")]
    public string LinuxUrl { get; set; } = "";

    [JsonPropertyName("windows_sha256")]
    public string WindowsSha256 { get; set; } = "";

    [JsonPropertyName("release_notes")]
    public string ReleaseNotes { get; set; } = "";

    [JsonPropertyName("required")]
    public bool Required { get; set; }

    /// <summary>
    /// Per-platform expansion added by the backend. The Windows entry is the
    /// authoritative source for what to install on this client — the legacy
    /// top-level <see cref="Version"/> is a cross-platform max and can drift
    /// ahead of <see cref="WindowsUrl"/>'s actual version (root cause of the
    /// "current 2.2.10, install 2.3.1, still 2.2.10" loop). Null on legacy
    /// backends; the client falls back to the top-level fields then.
    /// </summary>
    [JsonPropertyName("platforms")]
    public Dictionary<string, PlatformRelease>? Platforms { get; set; }
}

public class PlatformRelease
{
    [JsonPropertyName("version")]
    public string Version { get; set; } = "";

    [JsonPropertyName("url")]
    public string Url { get; set; } = "";

    [JsonPropertyName("sha256")]
    public string Sha256 { get; set; } = "";

    [JsonPropertyName("notes")]
    public string Notes { get; set; } = "";

    [JsonPropertyName("required")]
    public bool Required { get; set; }
}

/// <summary>
/// Decisions we make about an <see cref="UpdateInfo"/> before any download or
/// UI happens: which release this platform should install, what its artifact
/// hash is, and whether it is newer than what is running.
/// </summary>
public static class UpdateManifest
{
    /// <summary>
    /// Effective artifact hash for <paramref name="platform"/>: the
    /// per-platform entry wins, falling back to the legacy top-level field.
    /// Lowercased; empty when the release predates hash publishing, which
    /// callers treat as "install unverified with a warning" for back-compat.
    /// </summary>
    /// <remarks>
    /// Takes the platform rather than assuming Windows so the macOS/Linux
    /// clients can share this logic when their manifests grow the same shape.
    /// The legacy top-level fallback is still Windows-specific because that is
    /// the only top-level hash field the backend ever emitted.
    /// </remarks>
    public static string ResolveSha256(UpdateInfo info, string platform)
    {
        var fromPlatform = info.Platforms != null
            && info.Platforms.TryGetValue(platform, out var pin) && pin != null
            ? pin.Sha256 : null;
        var hash = !string.IsNullOrEmpty(fromPlatform) ? fromPlatform : info.WindowsSha256;
        return (hash ?? "").Trim().ToLowerInvariant();
    }

    /// <summary>
    /// Collapses the manifest onto a single platform's release.
    /// <see cref="UpdateInfo.Version"/> / <see cref="UpdateInfo.WindowsUrl"/>
    /// / notes / required are taken from <c>platforms[platform]</c> when that
    /// entry is present and well-formed. That entry is tied to the same DB
    /// row as <c>windows_url</c>, so the two can never drift — unlike the
    /// legacy envelope which used a cross-platform max for `version`. Falls
    /// through unchanged when the backend is too old to emit the
    /// <c>platforms</c> map (or the platform-specific entry is empty), so
    /// older deployments keep working.
    /// </summary>
    public static UpdateInfo NormalizeForPlatform(UpdateInfo raw, string platform)
    {
        if (raw.Platforms == null) return raw;
        if (!raw.Platforms.TryGetValue(platform, out var pin) || pin == null) return raw;
        if (string.IsNullOrEmpty(pin.Version)) return raw;

        return new UpdateInfo
        {
            Version = pin.Version,
            WindowsUrl = !string.IsNullOrEmpty(pin.Url) ? pin.Url : raw.WindowsUrl,
            WindowsSha256 = !string.IsNullOrEmpty(pin.Sha256) ? pin.Sha256 : raw.WindowsSha256,
            MacUrl = raw.MacUrl,
            LinuxUrl = raw.LinuxUrl,
            ReleaseNotes = !string.IsNullOrEmpty(pin.Notes) ? pin.Notes : raw.ReleaseNotes,
            Required = pin.Required || raw.Required,
            Platforms = raw.Platforms,
        };
    }

    /// <summary>
    /// Dotted-numeric version comparison. Missing components count as 0, so
    /// "2.3" and "2.3.0" compare equal and non-numeric junk degrades to 0
    /// rather than throwing.
    /// </summary>
    public static bool IsNewer(string remote, string local)
    {
        var r = remote.Split('.').Select(s => int.TryParse(s, out var n) ? n : 0).ToArray();
        var l = local.Split('.').Select(s => int.TryParse(s, out var n) ? n : 0).ToArray();
        var len = Math.Max(r.Length, l.Length);
        for (int i = 0; i < len; i++)
        {
            var rv = i < r.Length ? r[i] : 0;
            var lv = i < l.Length ? l[i] : 0;
            if (rv > lv) return true;
            if (rv < lv) return false;
        }
        return false;
    }
}
