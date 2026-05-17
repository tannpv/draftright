using System;
using System.IO;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;
using DraftRightWindows.Models;

namespace DraftRightWindows.Services;

/// <summary>
/// Manages authentication state and persists tokens using DPAPI encryption.
/// Tokens are stored in %LOCALAPPDATA%\DraftRight\auth.json, encrypted at rest.
/// </summary>
public sealed class AuthService
{
    private string? _accessToken;
    private string? _refreshToken;
    private string? _email;

    private static readonly string StorageDir =
        Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "DraftRight");

    private static readonly string AuthFilePath = Path.Combine(StorageDir, "auth.json");

    /// <summary>True when an access token is present in memory.</summary>
    public bool IsLoggedIn => !string.IsNullOrEmpty(_accessToken);

    /// <summary>Current access token (null if not logged in).</summary>
    public string? AccessToken => _accessToken;

    /// <summary>Current refresh token (null if not logged in).</summary>
    public string? RefreshToken => _refreshToken;

    /// <summary>Current user email (empty string if not logged in).</summary>
    public string CurrentEmail => _email ?? string.Empty;

    // ── Public API ──────────────────────────────────────────

    /// <summary>
    /// Raised after tokens are successfully persisted. Subscribers can use it
    /// to reset session-expired UI state — see <c>App.cs</c> wiring.
    /// </summary>
    public event Action? TokensSaved;

    /// <summary>
    /// Persists tokens and email to encrypted storage and updates in-memory state.
    /// </summary>
    public void SaveTokens(string accessToken, string? refreshToken, string? email = null)
    {
        DRLogger.Log(
            $"SaveTokens: email={email ?? "(null)"} access={Mask(accessToken)} refresh={Mask(refreshToken)}",
            DRLogger.Category.AUTH);

        _accessToken = accessToken;
        _refreshToken = refreshToken;
        _email = email;

        Directory.CreateDirectory(StorageDir);

        var payload = new StoredTokens
        {
            AccessToken = accessToken,
            RefreshToken = refreshToken,
            Email = email
        };

        try
        {
            var json = JsonSerializer.Serialize(payload);
            var plainBytes = Encoding.UTF8.GetBytes(json);
            var encrypted = ProtectedData.Protect(plainBytes, null, DataProtectionScope.CurrentUser);

            File.WriteAllBytes(AuthFilePath, encrypted);
            DRLogger.Log($"SaveTokens: persisted to {AuthFilePath} ({encrypted.Length} bytes encrypted)",
                DRLogger.Category.AUTH);
        }
        catch (Exception ex)
        {
            DRLogger.Log($"SaveTokens: failed to persist — {ex.GetType().Name}: {ex.Message}",
                DRLogger.Category.AUTH);
            throw;
        }

        TokensSaved?.Invoke();
    }

    /// <summary>
    /// Clears tokens from memory and disk.
    /// </summary>
    public void ClearTokens()
    {
        DRLogger.Log($"ClearTokens: had email={_email ?? "(null)"} access={Mask(_accessToken)}",
            DRLogger.Category.AUTH);

        _accessToken = null;
        _refreshToken = null;
        _email = null;

        try
        {
            if (File.Exists(AuthFilePath))
            {
                File.Delete(AuthFilePath);
                DRLogger.Log($"ClearTokens: deleted {AuthFilePath}", DRLogger.Category.AUTH);
            }
        }
        catch (Exception ex)
        {
            DRLogger.Log($"ClearTokens: failed to delete auth file — {ex.GetType().Name}: {ex.Message}",
                DRLogger.Category.AUTH);
        }
    }

    /// <summary>
    /// Restores a previous session by loading encrypted tokens from disk.
    /// Call this at app startup.
    /// </summary>
    public bool RestoreSession()
    {
        DRLogger.Log("RestoreSession: starting", DRLogger.Category.AUTH);
        try
        {
            if (!File.Exists(AuthFilePath))
            {
                DRLogger.Log($"RestoreSession: no auth file at {AuthFilePath}", DRLogger.Category.AUTH);
                return false;
            }

            var encrypted = File.ReadAllBytes(AuthFilePath);
            var decrypted = ProtectedData.Unprotect(encrypted, null, DataProtectionScope.CurrentUser);
            var json = Encoding.UTF8.GetString(decrypted);
            var stored = JsonSerializer.Deserialize<StoredTokens>(json);

            if (stored is null || string.IsNullOrEmpty(stored.AccessToken))
            {
                DRLogger.Log("RestoreSession: stored payload empty / missing access token",
                    DRLogger.Category.AUTH);
                return false;
            }

            _accessToken = stored.AccessToken;
            _refreshToken = stored.RefreshToken;
            _email = stored.Email;
            DRLogger.Log(
                $"RestoreSession: restored email={_email ?? "(null)"} access={Mask(_accessToken)} refresh={Mask(_refreshToken)}",
                DRLogger.Category.AUTH);
            return true;
        }
        catch (Exception ex)
        {
            DRLogger.Log($"RestoreSession: failed — {ex.GetType().Name}: {ex.Message}",
                DRLogger.Category.AUTH);
            return false;
        }
    }

    /// <summary>Last 4 characters of a token for logs. Returns "(null)" / "(empty)"
    /// for missing values so log entries are never silently empty.</summary>
    private static string Mask(string? token)
    {
        if (token is null) return "(null)";
        if (token.Length == 0) return "(empty)";
        if (token.Length <= 4) return "***";
        return "***" + token.Substring(token.Length - 4);
    }

    // ── Internal model ──────────────────────────────────────

    private sealed class StoredTokens
    {
        public string AccessToken { get; set; } = string.Empty;
        public string? RefreshToken { get; set; }
        public string? Email { get; set; }
    }
}
