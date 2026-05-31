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
        DRLogger.Log($"ApiClient: constructed baseUrl={_baseUrl}", DRLogger.Category.API);
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
    public void SetBaseUrl(string url)
    {
        var normalized = url.StripTrailingSlash();
        DRLogger.Log($"SetBaseUrl: {_baseUrl} → {normalized}", DRLogger.Category.API);
        _baseUrl = normalized;
    }

    public void SetToken(string token)
    {
        _http.DefaultRequestHeaders.Authorization =
            new AuthenticationHeaderValue("Bearer", token);
        DRLogger.Log($"SetToken: bearer={MaskToken(token)}", DRLogger.Category.API);
    }

    /// <summary>
    /// Clears the Bearer authorization header.
    /// </summary>
    public void ClearToken()
    {
        _http.DefaultRequestHeaders.Authorization = null;
        DRLogger.Log("ClearToken: bearer cleared", DRLogger.Category.API);
    }

    private static string MaskToken(string? token)
    {
        if (token is null) return "(null)";
        if (token.Length == 0) return "(empty)";
        if (token.Length <= 4) return "***";
        return "***" + token.Substring(token.Length - 4);
    }

    // ── Auth ────────────────────────────────────────────────

    public async Task<AuthResponse> LoginAsync(string email, string password)
    {
        DRLogger.Log($"LoginAsync: email={email}", DRLogger.Category.API);
        var body = new LoginRequest { Email = email, Password = password };
        return await PostAsync<AuthResponse>("/auth/login", body, autoRefresh: false);
    }

    public async Task<AuthResponse> RegisterAsync(string email, string password, string name)
    {
        DRLogger.Log($"RegisterAsync: email={email} name={name}", DRLogger.Category.API);
        var body = new RegisterRequest { Email = email, Password = password, Name = name };
        return await PostAsync<AuthResponse>("/auth/register", body, autoRefresh: false);
    }

    /// <summary>
    /// Exchange a third-party identity token (e.g. a Google id_token from
    /// <see cref="GoogleOAuth.AuthenticateAsync"/>) for a DraftRight session.
    /// Mirrors the Flutter mobile client's social-login flow so the backend's
    /// /auth/social user creation/linking is shared across platforms.
    /// </summary>
    public async Task<AuthResponse> SocialLoginAsync(
        string provider, string idToken, string? name = null, string? email = null, string? avatarUrl = null)
    {
        DRLogger.Log($"SocialLoginAsync: provider={provider}", DRLogger.Category.API);
        var body = new
        {
            provider,
            id_token = idToken,
            name,
            email,
            avatar_url = avatarUrl,
        };
        return await PostAsync<AuthResponse>("/auth/social", body, autoRefresh: false);
    }

    /// <summary>
    /// Exchanges a refresh token for a fresh access/refresh pair.
    /// Does not auto-retry on 401 — a 401 here means the refresh token itself is invalid.
    /// </summary>
    public async Task<AuthResponse> RefreshAsync(string refreshToken)
    {
        DRLogger.Log($"RefreshAsync: refresh={MaskToken(refreshToken)}", DRLogger.Category.API);
        var body = new { refresh_token = refreshToken };
        return await PostAsync<AuthResponse>("/auth/refresh", body, autoRefresh: false);
    }

    // ── Rewrite ─────────────────────────────────────────────

    public async Task<RewriteResponse> RewriteAsync(string text, string tone, string? targetLanguage = null)
    {
        DRLogger.Log(
            $"RewriteAsync: tone={tone} targetLanguage={targetLanguage ?? "(none)"} textLen={text.Length}",
            DRLogger.Category.API);
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
        DRLogger.Log("GetSubscriptionAsync", DRLogger.Category.API);
        return await GetAsync<SubscriptionResponse>("/subscription");
    }

    // ── Payment ─────────────────────────────────────────────

    public async Task<List<PaymentMethodKind>> ListPaymentMethodsAsync()
    {
        DRLogger.Log("ListPaymentMethodsAsync", DRLogger.Category.API);
        var raw = await GetAsync<PaymentMethodsResponse>("/payment/methods", autoRefresh: false);
        var kinds = new List<PaymentMethodKind>();
        foreach (var s in raw.Methods)
        {
            var k = PaymentMethodKindExtensions.FromWire(s);
            if (k.HasValue) kinds.Add(k.Value);
        }
        return kinds;
    }

    /// <summary>Public plan catalog — unauthenticated, returns a JSON array.</summary>
    public async Task<List<PlanRow>> ListPlansAsync()
    {
        DRLogger.Log("ListPlansAsync", DRLogger.Category.API);
        // /plans returns a List at the root, not an object; can't go
        // through GetAsync<T> which assumes object root.
        using var resp = await _http.GetAsync($"{_baseUrl}/plans");
        resp.EnsureSuccessStatusCode();
        var body = await resp.Content.ReadAsStringAsync();
        return JsonSerializer.Deserialize<List<PlanRow>>(body, JsonOptions) ?? new();
    }

    public async Task<CheckoutResult> CreateCheckoutAsync(string planId, PaymentMethodKind method)
    {
        DRLogger.Log($"CreateCheckoutAsync plan={planId} method={method.WireName()}", DRLogger.Category.API);
        var body = new { plan_id = planId, method = method.WireName() };
        // Use the auto-refresh wrapper but keep the raw JSON in our hands
        // because the response is a union shape; can't bind to one DTO.
        var json = await PostRawJsonAsync("/payment/checkout", body);
        using var doc = JsonDocument.Parse(json);
        return CheckoutResult.FromJson(doc.RootElement);
    }

    public async Task<PaymentStatusUpdate> GetPaymentStatusAsync(string referenceCode)
    {
        var raw = await GetAsync<PaymentStatusResponse>(
            $"/payment/status/{Uri.EscapeDataString(referenceCode)}",
            autoRefresh: false);
        return raw.ToUpdate();
    }

    public async Task<string> GetCustomerPortalUrlAsync()
    {
        DRLogger.Log("GetCustomerPortalUrlAsync", DRLogger.Category.API);
        var resp = await GetAsync<CustomerPortalResponse>("/lemonsqueezy/portal");
        return resp.Url;
    }

    // ── Health Check ────────────────────────────────────────

    public async Task<BackendStatus> CheckHealthAsync()
    {
        var sw = System.Diagnostics.Stopwatch.StartNew();
        try
        {
            // Step 1: Check /health for app identity
            using var cts = new System.Threading.CancellationTokenSource(TimeSpan.FromSeconds(5));

            using var healthRequest = new HttpRequestMessage(HttpMethod.Get, $"{_baseUrl}/health");
            using var healthResponse = await _http.SendAsync(healthRequest, cts.Token);
            DRLogger.Log(
                $"CheckHealthAsync: /health → {(int)healthResponse.StatusCode} {healthResponse.ReasonPhrase} ({sw.ElapsedMilliseconds}ms)",
                DRLogger.Category.API);

            if (!healthResponse.IsSuccessStatusCode)
                return BackendStatus.Offline;

            var healthBody = await healthResponse.Content.ReadAsStringAsync();
            using var doc = JsonDocument.Parse(healthBody);
            var app = doc.RootElement.TryGetProperty("app", out var appProp) ? appProp.GetString() : null;

            if (app != "draftright")
            {
                DRLogger.Warn($"CheckHealthAsync: /health.app={app ?? "(null)"} (expected 'draftright') → WrongServer",
                    DRLogger.Category.API);
                return BackendStatus.WrongServer;
            }

            // Apply the admin-controlled log verbosity (only once we've
            // confirmed this is really a DraftRight backend, so a rogue server
            // can't silence our logs). Older backends omit the field → no-op.
            if (doc.RootElement.TryGetProperty("client_log_level", out var lvlProp)
                && lvlProp.ValueKind == JsonValueKind.String)
            {
                DRLogger.SetMinLevelFromServer(lvlProp.GetString());
            }

            // Step 2: Check /auth/me for login state. Goes through the
            // auto-refresh wrapper so an expired access token gets refreshed
            // before we mistakenly tell the tray "Not Logged In" — the bug
            // was: raw _http.SendAsync skipped OnUnauthorized, so health
            // ticks stayed on NotLoggedIn even though rewrite/subscription
            // calls (which DO use SendWithAutoRefresh) kept refreshing fine.
            using var authResponse = await SendAuthMeWithRefreshAsync(cts.Token);
            var result = authResponse.StatusCode switch
            {
                System.Net.HttpStatusCode.OK => BackendStatus.Connected,
                System.Net.HttpStatusCode.Unauthorized => BackendStatus.NotLoggedIn,
                _ => BackendStatus.Offline
            };
            DRLogger.Log(
                $"CheckHealthAsync: /auth/me → {(int)authResponse.StatusCode} → {result} (total {sw.ElapsedMilliseconds}ms)",
                DRLogger.Category.API);
            return result;
        }
        catch (Exception ex)
        {
            DRLogger.Warn(
                $"CheckHealthAsync: failed after {sw.ElapsedMilliseconds}ms — {ex.GetType().Name}: {ex.Message}",
                DRLogger.Category.API);
            return BackendStatus.Offline;
        }
    }

    /// <summary>
    /// Mirrors <see cref="SendWithAutoRefreshAsync{T}"/> but returns the raw
    /// <see cref="HttpResponseMessage"/> — used by <see cref="CheckHealthAsync"/>
    /// which cares only about the status code, not a deserialized body. Sends
    /// once, on 401 invokes <see cref="OnUnauthorized"/>, and retries exactly
    /// once with the refreshed bearer. Caller owns disposal of the returned
    /// response.
    /// </summary>
    private async Task<HttpResponseMessage> SendAuthMeWithRefreshAsync(
        System.Threading.CancellationToken ct)
    {
        var sw = System.Diagnostics.Stopwatch.StartNew();
        var first = await _http.GetAsync($"{_baseUrl}/auth/me", ct);
        DRLogger.Log(
            $"HTTP GET /auth/me → {(int)first.StatusCode} {first.ReasonPhrase} ({sw.ElapsedMilliseconds}ms)",
            DRLogger.Category.API);

        if (first.StatusCode != System.Net.HttpStatusCode.Unauthorized || OnUnauthorized == null)
            return first;

        DRLogger.Log("CheckHealthAsync: 401 from /auth/me, invoking OnUnauthorized for refresh",
            DRLogger.Category.API);
        first.Dispose();

        var refreshed = await OnUnauthorized();
        DRLogger.Log($"CheckHealthAsync: OnUnauthorized returned {refreshed}", DRLogger.Category.API);
        if (!refreshed)
        {
            // Refresh failed — return a synthetic 401 so the caller still
            // maps to NotLoggedIn rather than treating it as Offline.
            return new HttpResponseMessage(System.Net.HttpStatusCode.Unauthorized);
        }

        sw.Restart();
        var second = await _http.GetAsync($"{_baseUrl}/auth/me", ct);
        DRLogger.Log(
            $"HTTP GET retry /auth/me → {(int)second.StatusCode} {second.ReasonPhrase} ({sw.ElapsedMilliseconds}ms)",
            DRLogger.Category.API);
        return second;
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
    /// POSTs a JSON payload and returns the raw response body as a
    /// string.  Used for endpoints whose JSON shape is a discriminated
    /// union and can't be bound to one DTO (e.g. /payment/checkout).
    /// Still honors auto-refresh on 401.
    /// </summary>
    private async Task<string> PostRawJsonAsync(string path, object payload, bool autoRefresh = true)
    {
        var json = JsonSerializer.Serialize(payload, JsonOptions);
        var sw = System.Diagnostics.Stopwatch.StartNew();
        HttpResponseMessage response = await SendRawAsync();
        try
        {
            if (autoRefresh
                && response.StatusCode == System.Net.HttpStatusCode.Unauthorized
                && OnUnauthorized != null)
            {
                response.Dispose();
                var refreshed = await OnUnauthorized();
                if (!refreshed)
                {
                    throw new ApiException(
                        "API 401 Unauthorized: Session expired. Please sign in again.",
                        System.Net.HttpStatusCode.Unauthorized);
                }
                sw.Restart();
                response = await SendRawAsync();
            }
            var body = await response.Content.ReadAsStringAsync();
            if (!response.IsSuccessStatusCode)
            {
                throw new ApiException(
                    $"API {(int)response.StatusCode} {response.ReasonPhrase}: {body}",
                    response.StatusCode);
            }
            return body;
        }
        finally
        {
            response.Dispose();
        }

        Task<HttpResponseMessage> SendRawAsync()
        {
            var content = new StringContent(json, Encoding.UTF8, "application/json");
            return _http.PostAsync($"{_baseUrl}{path}", content);
        }
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
        var sw = System.Diagnostics.Stopwatch.StartNew();
        var response = await send();
        DRLogger.Log(
            $"HTTP {response.RequestMessage?.Method?.Method} {response.RequestMessage?.RequestUri} → {(int)response.StatusCode} {response.ReasonPhrase} ({sw.ElapsedMilliseconds}ms)",
            DRLogger.Category.API);
        try
        {
            if (autoRefresh
                && response.StatusCode == System.Net.HttpStatusCode.Unauthorized
                && OnUnauthorized != null)
            {
                DRLogger.Log("Auto-refresh: 401 received, invoking OnUnauthorized callback",
                    DRLogger.Category.API);
                response.Dispose();
                response = null!;
                var refreshed = await OnUnauthorized();
                DRLogger.Log($"Auto-refresh: callback returned {refreshed}", DRLogger.Category.API);
                if (!refreshed)
                {
                    throw new ApiException(
                        "API 401 Unauthorized: Session expired. Please sign in again.",
                        System.Net.HttpStatusCode.Unauthorized);
                }
                sw.Restart();
                response = await send();
                DRLogger.Log(
                    $"HTTP retry {response.RequestMessage?.Method?.Method} {response.RequestMessage?.RequestUri} → {(int)response.StatusCode} {response.ReasonPhrase} ({sw.ElapsedMilliseconds}ms)",
                    DRLogger.Category.API);
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
            // Extract a user-facing message from the error body. NestJS
            // class-validator returns `{"message":["email must be an email"]}`
            // (string[]) for DTO failures and `{"message":"…"}` (string) for
            // most other errors — we have to handle both, otherwise the
            // array falls through to the raw JSON body and the user sees
            // stack-trace soup. See BUG-18 / BUG-19 (2026-05-29).
            string detail = body;
            try
            {
                using var doc = JsonDocument.Parse(body);
                if (doc.RootElement.TryGetProperty("message", out var msg))
                {
                    if (msg.ValueKind == JsonValueKind.String)
                    {
                        detail = msg.GetString() ?? body;
                    }
                    else if (msg.ValueKind == JsonValueKind.Array && msg.GetArrayLength() > 0)
                    {
                        // Join with "; " so multi-field validations show all problems.
                        var parts = new List<string>();
                        foreach (var el in msg.EnumerateArray())
                        {
                            var s = el.GetString();
                            if (!string.IsNullOrEmpty(s)) parts.Add(s);
                        }
                        if (parts.Count > 0) detail = string.Join("; ", parts);
                    }
                }
            }
            catch
            {
                /* body wasn't JSON — keep raw */
            }

            // Truncated body preview so error log entries don't bloat the log
            // with multi-KB stack traces from the backend.
            var preview = detail.Length > 200 ? detail.Substring(0, 200) + "…" : detail;
            DRLogger.Error(
                $"HandleResponse: non-success {(int)response.StatusCode} {response.ReasonPhrase} body={preview}",
                DRLogger.Category.API);

            throw new ApiException(
                $"API {(int)response.StatusCode} {response.ReasonPhrase}: {detail}",
                response.StatusCode,
                serverMessage: detail);
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

    /// <summary>
    /// User-facing message extracted from the server's response body —
    /// e.g. "email must be an email" without the "API 400 Bad Request:"
    /// prefix or surrounding JSON. UI surfaces should prefer this over
    /// <see cref="Exception.Message"/> so users don't see stack-trace soup.
    /// Falls back to <see cref="Exception.Message"/> when null.
    /// </summary>
    public string? ServerMessage { get; }

    public ApiException(string message, System.Net.HttpStatusCode statusCode, string? serverMessage = null)
        : base(message)
    {
        StatusCode = statusCode;
        ServerMessage = serverMessage;
    }
}
