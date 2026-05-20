using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.Json;
using System.Text.Json.Serialization;
using DraftRightWindows.Models;

namespace DraftRightWindows.Services;

/// <summary>
/// Persists user preferences as JSON in %LOCALAPPDATA%\DraftRight\settings.json.
/// </summary>
public sealed class SettingsService
{
    private static readonly string StorageDir =
        Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "DraftRight");

    private static readonly string SettingsFilePath = Path.Combine(StorageDir, "settings.json");

    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        WriteIndented = true,
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull
    };

    // ── Settings properties ─────────────────────────────────

    /// <summary>Backend API base URL.</summary>
    public string BackendUrl { get; set; } = Constants.DefaultBackendUrl;

    /// <summary>Win32 modifier flags for the global hotkey (MOD_CONTROL | MOD_SHIFT = 0x0006).</summary>
    public int HotkeyModifiers { get; set; } = 0x0002 | 0x0004; // Ctrl + Shift

    /// <summary>Virtual-key code for the hotkey letter (default 'R' = 0x52).</summary>
    public int HotkeyKey { get; set; } = 0x52; // VK_R

    /// <summary>Target language for translation rewrites.</summary>
    public string TranslateLanguage { get; set; } = "Vietnamese";

    /// <summary>Whether the app should launch at Windows startup.</summary>
    public bool AutoStart { get; set; }

    /// <summary>List of enabled tone API values (default: all tones).</summary>
    public List<string> EnabledTones { get; set; } = Enum.GetValues<Tone>().Select(t => t.ApiValue()).ToList();

    /// <summary>Default tone API value for auto-run (empty = none).</summary>
    public string DefaultTone { get; set; } = "";

    /// <summary>Interaction mode. Defaults to Advanced so existing users see no behavior change.</summary>
    public AppMode AppMode { get; set; } = AppMode.Advanced;

    /// <summary>Preset tone used by One-Click mode. Stored as the tone's API value.</summary>
    public string OneClickTone { get; set; } = "polished";

    /// <summary>When false, DRLogger.Log short-circuits — no file writes.</summary>
    public bool LoggingEnabled { get; set; } = true;

    /// <summary>The app version that last ran on this machine. Drives the
    /// one-time "What's New" notice shown on the first launch after an update
    /// applies. Empty on a fresh install (no notice shown).</summary>
    public string LastSeenVersion { get; set; } = "";

    // ── Persistence ─────────────────────────────────────────

    /// <summary>
    /// Loads settings from disk. Returns silently with defaults if the file
    /// does not exist or is corrupt.
    /// </summary>
    public void Load()
    {
        try
        {
            if (!File.Exists(SettingsFilePath))
                return;

            var json = File.ReadAllText(SettingsFilePath);
            var loaded = JsonSerializer.Deserialize<SettingsData>(json, JsonOptions);

            if (loaded is null)
                return;

            BackendUrl = loaded.BackendUrl ?? BackendUrl;
            HotkeyModifiers = loaded.HotkeyModifiers ?? HotkeyModifiers;
            HotkeyKey = loaded.HotkeyKey ?? HotkeyKey;
            TranslateLanguage = loaded.TranslateLanguage ?? TranslateLanguage;
            AutoStart = loaded.AutoStart ?? AutoStart;
            if (loaded.EnabledTones != null)
                EnabledTones = loaded.EnabledTones;
            DefaultTone = loaded.DefaultTone ?? DefaultTone;
            AppMode = AppModeExtensions.FromApiValue(loaded.AppMode);
            OneClickTone = loaded.OneClickTone ?? OneClickTone;
            LoggingEnabled = loaded.LoggingEnabled ?? LoggingEnabled;
            LastSeenVersion = loaded.LastSeenVersion ?? LastSeenVersion;
        }
        catch
        {
            // Use defaults on any read/parse error
        }
    }

    /// <summary>
    /// Persists current settings to disk.
    /// </summary>
    public void Save()
    {
        try
        {
            Directory.CreateDirectory(StorageDir);

            var data = new SettingsData
            {
                BackendUrl = BackendUrl,
                HotkeyModifiers = HotkeyModifiers,
                HotkeyKey = HotkeyKey,
                TranslateLanguage = TranslateLanguage,
                AutoStart = AutoStart,
                EnabledTones = EnabledTones,
                DefaultTone = DefaultTone,
                AppMode = AppMode.ApiValue(),
                OneClickTone = OneClickTone,
                LoggingEnabled = LoggingEnabled,
                LastSeenVersion = LastSeenVersion
            };

            var json = JsonSerializer.Serialize(data, JsonOptions);
            File.WriteAllText(SettingsFilePath, json);
        }
        catch
        {
            // Best-effort save — don't crash the app
        }
    }

    // ── Internal DTO ────────────────────────────────────────

    /// <summary>
    /// Nullable DTO so missing keys in JSON fall back to current defaults.
    /// </summary>
    private sealed class SettingsData
    {
        public string? BackendUrl { get; set; }
        public int? HotkeyModifiers { get; set; }
        public int? HotkeyKey { get; set; }
        public string? TranslateLanguage { get; set; }
        public bool? AutoStart { get; set; }
        public List<string>? EnabledTones { get; set; }
        public string? DefaultTone { get; set; }
        public string? AppMode { get; set; }
        public string? OneClickTone { get; set; }
        public bool? LoggingEnabled { get; set; }
        public string? LastSeenVersion { get; set; }
    }
}
