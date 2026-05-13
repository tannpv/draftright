using System;
using System.Collections.Generic;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;

namespace DraftRightWindows.Services;

/// <summary>
/// Posts feature-request feedback to the DraftRight backend's <c>POST /feedback</c>
/// endpoint as <c>application/json</c>. Anonymous and authenticated calls are
/// both supported.
///
/// Mirrors <see cref="BugReportService"/> for URL/token resolution and
/// <see cref="SubmitResult"/> shape.
/// </summary>
public static class FeedbackService
{
    private static readonly HttpClient _http = new() { Timeout = TimeSpan.FromSeconds(30) };
    private const string DefaultBackendUrl = "https://api.draftright.info";

    /// <summary>
    /// Result of a feedback submission. <see cref="Success"/> indicates HTTP 2xx;
    /// <see cref="ErrorMessage"/> carries the failure reason otherwise.
    /// </summary>
    public sealed record SubmitResult(bool Success, string? Id = null, string? ErrorMessage = null);

    /// <summary>
    /// POSTs a feature request to <c>/feedback</c>.
    /// </summary>
    /// <param name="title">Short title for the request (max 80 chars, enforced by caller).</param>
    /// <param name="targetPlatform">One of: playground|mobile|windows|mac|linux.</param>
    /// <param name="description">Longer description of the feature.</param>
    /// <param name="userEmail">Optional. Sent only when <paramref name="authToken"/> is absent.</param>
    /// <param name="authToken">Optional. Bearer JWT — when present server attaches user_id.</param>
    /// <param name="backendUrlOverride">Optional. Override base URL (tests / dev). Defaults to settings.</param>
    public static async Task<SubmitResult> SubmitAsync(
        string title,
        string targetPlatform,
        string description,
        string? userEmail = null,
        string? authToken = null,
        string? backendUrlOverride = null)
    {
        if (string.IsNullOrWhiteSpace(title))
            return new SubmitResult(false, ErrorMessage: "Title is required.");

        if (string.IsNullOrWhiteSpace(description))
            return new SubmitResult(false, ErrorMessage: "Description is required.");

        // Resolve base URL: explicit override > current Settings > hardcoded default.
        // App.Settings is null in unit/standalone scenarios, so guard before reaching it.
        var baseUrl = backendUrlOverride;
        if (string.IsNullOrWhiteSpace(baseUrl))
        {
            try { baseUrl = App.Settings?.BackendUrl; } catch { baseUrl = null; }
        }
        if (string.IsNullOrWhiteSpace(baseUrl)) baseUrl = DefaultBackendUrl;
        baseUrl = baseUrl!.TrimEnd('/');

        try
        {
            var payload = new Dictionary<string, object?>
            {
                ["kind"] = "feature",
                ["title"] = title.Trim(),
                ["target_platform"] = targetPlatform,
                ["description"] = description.Trim(),
                ["source"] = "windows-app",
            };

            if (string.IsNullOrWhiteSpace(authToken) && !string.IsNullOrWhiteSpace(userEmail))
                payload["user_email"] = userEmail!.Trim();

            using var request = new HttpRequestMessage(HttpMethod.Post, $"{baseUrl}/feedback")
            {
                Content = new StringContent(
                    JsonSerializer.Serialize(payload),
                    Encoding.UTF8,
                    "application/json"),
            };

            if (!string.IsNullOrEmpty(authToken))
                request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", authToken);

            using var response = await _http.SendAsync(request).ConfigureAwait(false);
            var body = await response.Content.ReadAsStringAsync().ConfigureAwait(false);

            if (!response.IsSuccessStatusCode)
            {
                string detail;
                try
                {
                    using var doc = JsonDocument.Parse(body);
                    if (doc.RootElement.TryGetProperty("error", out var err))
                        detail = err.GetString() ?? body;
                    else if (doc.RootElement.TryGetProperty("message", out var msg))
                        detail = msg.GetString() ?? body;
                    else
                        detail = body;
                }
                catch
                {
                    detail = body;
                }
                return new SubmitResult(
                    false,
                    ErrorMessage: $"Server {(int)response.StatusCode}: {detail}");
            }

            string? id = null;
            try
            {
                using var doc = JsonDocument.Parse(body);
                if (doc.RootElement.TryGetProperty("id", out var idProp))
                    id = idProp.GetString();
            }
            catch
            {
                // 2xx but unparseable body — still treat as success.
            }

            return new SubmitResult(true, Id: id);
        }
        catch (TaskCanceledException)
        {
            return new SubmitResult(false, ErrorMessage: "Request timed out. Check your connection and retry.");
        }
        catch (Exception ex)
        {
            return new SubmitResult(false, ErrorMessage: ex.Message);
        }
    }
}
