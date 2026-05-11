using System;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;
using DraftRightWindows.Helpers;
using DraftRightWindows.Models;

namespace DraftRightWindows.Services;

public enum BackendStatus { Connected, NotLoggedIn, Offline, WrongServer }

/// <summary>
/// HttpClient wrapper for the DraftRight backend API.
/// </summary>
public sealed class ApiClient : IDisposable
{
    private readonly HttpClient _http;
    private string _baseUrl;

    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
        PropertyNameCaseInsensitive = true
    };

    public ApiClient(string backendUrl)
    {
        _baseUrl = backendUrl.StripTrailingSlash();
        _http = new HttpClient
        {
            Timeout = TimeSpan.FromSeconds(30)
        };
    }

    /// <summary>
    /// Invoked when an authenticated request receives 401. Implementations
    /// should attempt to refresh the access token (calling RefreshAsync) and
    /// return true if the retry should proceed, false to surface the 401 to
    /// the caller. Single-flight: parallel 401s are not coordinated here, so
    /// the callback may be invoked concurrently — keep it idempotent or guard
    /// it externally.
    /// </summary>
    public Func<Task<bool>>? OnUnauthorized { get; set; }

    /// <summary>
    /// Sets the Bearer authorization header for subsequent requests.
    /// </summary>
    public void SetBaseUrl(string url) => _baseUrl = url.StripTrailingSlash();

    public void SetToken(string token)
    {
        _http.DefaultRequestHeaders.Authorization =
            new AuthenticationHeaderValue("Bearer", token);
    }

    /// <summary>
    /// Clears the Bearer authorization header.
    /// </summary>
    public void ClearToken()
    {
        _http.DefaultRequestHeaders.Authorization = null;
    }

    // ── Auth ────────────────────────────────────────────────

    public async Task<AuthResponse> LoginAsync(string email, string password)
    {
        var body = new LoginRequest { Email = email, Password = password };
        return await PostAsync<AuthResponse>("/auth/login", body, autoRefresh: false);
    }

    public async Task<AuthResponse> RegisterAsync(string email, string password, string name)
    {
        var body = new RegisterRequest { Email = email, Password = password, Name = name };
        return await PostAsync<AuthResponse>("/auth/register", body, autoRefresh: false);
    }

    /// <summary>
    /// Exchanges a refresh token for a fresh access/refresh pair.
    /// Does not auto-retry on 401 — a 401 here means the refresh token itself is invalid.
    /// </summary>
    public async Task<AuthResponse> RefreshAsync(string refreshToken)
    {
        var body = new { refresh_token = refreshToken };
        return await PostAsync<AuthResponse>("/auth/refresh", body, autoRefresh: false);
    }

    // ── Rewrite ─────────────────────────────────────────────

    public async Task<RewriteResponse> RewriteAsync(string text, string tone, string? targetLanguage = null)
    {
        var body = new RewriteRequest
        {
            Text = text,
            Tone = tone,
            TargetLanguage = targetLanguage
        };
        return await PostAsync<RewriteResponse>("/rewrite", body);
    }

    // ── Subscription ────────────────────────────────────────

    public async Task<SubscriptionResponse> GetSubscriptionAsync()
    {
        return await GetAsync<SubscriptionResponse>("/subscription");
    }

    // ── Health Check ────────────────────────────────────────

    public async Task<BackendStatus> CheckHealthAsync()
    {
        try
        {
            // Step 1: Check /health for app identity
            using var cts = new System.Threading.CancellationTokenSource(TimeSpan.FromSeconds(5));

            using var healthRequest = new HttpRequestMessage(HttpMethod.Get, $"{_baseUrl}/health");
            using var healthResponse = await _http.SendAsync(healthRequest, cts.Token);

            if (!healthResponse.IsSuccessStatusCode)
                return BackendStatus.Offline;

            var healthBody = await healthResponse.Content.ReadAsStringAsync();
            using var doc = JsonDocument.Parse(healthBody);
            var app = doc.RootElement.TryGetProperty("app", out var appProp) ? appProp.GetString() : null;

            if (app != "draftright")
                return BackendStatus.WrongServer;

            // Step 2: Check /auth/me for login state
            using var authRequest = new HttpRequestMessage(HttpMethod.Get, $"{_baseUrl}/auth/me");
            authRequest.Headers.Authorization = _http.DefaultRequestHeaders.Authorization;
            using var authResponse = await _http.SendAsync(authRequest, cts.Token);

            return authResponse.StatusCode switch
            {
                System.Net.HttpStatusCode.OK => BackendStatus.Connected,
                System.Net.HttpStatusCode.Unauthorized => BackendStatus.NotLoggedIn,
                _ => BackendStatus.Offline
            };
        }
        catch
        {
            return BackendStatus.Offline;
        }
    }

    // ── Helpers ─────────────────────────────────────────────

    private async Task<T> PostAsync<T>(string path, object payload, bool autoRefresh = true)
    {
        var json = JsonSerializer.Serialize(payload, JsonOptions);
        return await SendWithAutoRefreshAsync<T>(
            () =>
            {
                var content = new StringContent(json, Encoding.UTF8, "application/json");
                return _http.PostAsync($"{_baseUrl}{path}", content);
            },
            autoRefresh);
    }

    private async Task<T> GetAsync<T>(string path, bool autoRefresh = true)
    {
        return await SendWithAutoRefreshAsync<T>(
            () => _http.GetAsync($"{_baseUrl}{path}"),
            autoRefresh);
    }

    /// <summary>
    /// Sends the request once; if it 401s and auto-refresh is on (and an
    /// OnUnauthorized callback is wired), invokes the callback and retries
    /// exactly once. If the callback returns false, surfaces a clear
    /// session-expired error rather than the raw 401.
    /// </summary>
    private async Task<T> SendWithAutoRefreshAsync<T>(
        Func<Task<HttpResponseMessage>> send,
        bool autoRefresh)
    {
        var response = await send();
        try
        {
            if (autoRefresh
                && response.StatusCode == System.Net.HttpStatusCode.Unauthorized
                && OnUnauthorized != null)
            {
                response.Dispose();
                response = null!;
                var refreshed = await OnUnauthorized();
                if (!refreshed)
                {
                    throw new ApiException(
                        "API 401 Unauthorized: Session expired. Please sign in again.",
                        System.Net.HttpStatusCode.Unauthorized);
                }
                response = await send();
            }
            return await HandleResponse<T>(response);
        }
        finally
        {
            response?.Dispose();
        }
    }

    private static async Task<T> HandleResponse<T>(HttpResponseMessage response)
    {
        var body = await response.Content.ReadAsStringAsync();

        if (!response.IsSuccessStatusCode)
        {
            // Try to extract a message from the error body
            string detail;
            try
            {
                using var doc = JsonDocument.Parse(body);
                detail = doc.RootElement.TryGetProperty("message", out var msg)
                    ? msg.GetString() ?? body
                    : body;
            }
            catch
            {
                detail = body;
            }

            throw new ApiException(
                $"API {(int)response.StatusCode} {response.ReasonPhrase}: {detail}",
                response.StatusCode);
        }

        return JsonSerializer.Deserialize<T>(body, JsonOptions)
            ?? throw new ApiException("Received null response from API.", response.StatusCode);
    }

    public void Dispose() => _http.Dispose();
}

/// <summary>
/// Exception thrown when the DraftRight API returns an error.
/// </summary>
public class ApiException : Exception
{
    public System.Net.HttpStatusCode StatusCode { get; }

    public ApiException(string message, System.Net.HttpStatusCode statusCode)
        : base(message)
    {
        StatusCode = statusCode;
    }
}
