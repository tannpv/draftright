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

        var json = JsonSerializer.Serialize(payload);
        var plainBytes = Encoding.UTF8.GetBytes(json);
        var encrypted = ProtectedData.Protect(plainBytes, null, DataProtectionScope.CurrentUser);

        File.WriteAllBytes(AuthFilePath, encrypted);

        TokensSaved?.Invoke();
    }

    /// <summary>
    /// Clears tokens from memory and disk.
    /// </summary>
    public void ClearTokens()
    {
        _accessToken = null;
        _refreshToken = null;
        _email = null;

        try
        {
            if (File.Exists(AuthFilePath))
                File.Delete(AuthFilePath);
        }
        catch
        {
            // Best-effort cleanup
        }
    }

    /// <summary>
    /// Restores a previous session by loading encrypted tokens from disk.
    /// Call this at app startup.
    /// </summary>
    public bool RestoreSession()
    {
        try
        {
            if (!File.Exists(AuthFilePath))
                return false;

            var encrypted = File.ReadAllBytes(AuthFilePath);
            var decrypted = ProtectedData.Unprotect(encrypted, null, DataProtectionScope.CurrentUser);
            var json = Encoding.UTF8.GetString(decrypted);
            var stored = JsonSerializer.Deserialize<StoredTokens>(json);

            if (stored is null || string.IsNullOrEmpty(stored.AccessToken))
                return false;

            _accessToken = stored.AccessToken;
            _refreshToken = stored.RefreshToken;
            _email = stored.Email;
            return true;
        }
        catch
        {
            // Corrupted or inaccessible file — start fresh
            return false;
        }
    }

    // ── Internal model ──────────────────────────────────────

    private sealed class StoredTokens
    {
        public string AccessToken { get; set; } = string.Empty;
        public string? RefreshToken { get; set; }
        public string? Email { get; set; }
    }
}
