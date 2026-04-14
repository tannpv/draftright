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
        return await PostAsync<AuthResponse>("/auth/login", body);
    }

    public async Task<AuthResponse> RegisterAsync(string email, string password, string name)
    {
        var body = new RegisterRequest { Email = email, Password = password, Name = name };
        return await PostAsync<AuthResponse>("/auth/register", body);
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

    private async Task<T> PostAsync<T>(string path, object payload)
    {
        var json = JsonSerializer.Serialize(payload, JsonOptions);
        using var content = new StringContent(json, Encoding.UTF8, "application/json");

        using var response = await _http.PostAsync($"{_baseUrl}{path}", content);
        return await HandleResponse<T>(response);
    }

    private async Task<T> GetAsync<T>(string path)
    {
        using var response = await _http.GetAsync($"{_baseUrl}{path}");
        return await HandleResponse<T>(response);
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
