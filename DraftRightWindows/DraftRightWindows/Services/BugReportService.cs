using System;
using System.Collections.Generic;
using System.IO;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;

namespace DraftRightWindows.Services;

/// <summary>
/// Submits bug reports to the DraftRight backend's <c>/bug-reports</c>
/// endpoint as <c>multipart/form-data</c>. Anonymous and authenticated
/// calls are both supported — when a JWT is supplied, the server attaches
/// the user_id automatically.
///
/// API contract: see /tmp/bug-report-contract.md.
/// Source field is hardcoded to <c>"windows"</c>.
/// </summary>
public static class BugReportService
{
    private static readonly HttpClient _http = new() { Timeout = TimeSpan.FromSeconds(60) };
    private const string DefaultBackendUrl = "https://api.draftright.info";

    /// <summary>
    /// Result of a bug report submission. <see cref="Success"/> indicates
    /// HTTP 2xx; <see cref="ErrorMessage"/> carries the failure reason on
    /// non-2xx, network errors, or local exceptions.
    /// </summary>
    public sealed record SubmitResult(bool Success, string? Id = null, string? ErrorMessage = null);

    /// <summary>
    /// POSTs a bug report.
    /// </summary>
    /// <param name="description">Required. Free-text description (min 10 chars enforced by caller).</param>
    /// <param name="screenshotPath">Optional. Path to a PNG/JPEG file on disk (max 5 MB).</param>
    /// <param name="userEmail">Optional. Reporter email — sent only when not signed in.</param>
    /// <param name="authToken">Optional. Bearer JWT — when present, /bug-reports records user_id.</param>
    /// <param name="context">Optional. Extra JSON context (route, plan, last action…).</param>
    /// <param name="backendUrlOverride">Optional. Override base URL (tests / dev). Defaults to settings.</param>
    public static async Task<SubmitResult> SubmitAsync(
        string description,
        string? screenshotPath = null,
        string? userEmail = null,
        string? authToken = null,
        Dictionary<string, object?>? context = null,
        string? backendUrlOverride = null)
    {
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

        FileStream? screenshotStream = null;
        try
        {
            using var form = new MultipartFormDataContent();

            // Required fields
            form.Add(new StringContent(description), "description");
            form.Add(new StringContent("windows"), "source");

            // App version — assembly version (Package.Current.Id.Version is unreachable
            // in unpackaged WinUI3 builds, which is what DraftRight ships today).
            var asm = System.Reflection.Assembly.GetExecutingAssembly();
            var appVersion = asm.GetName().Version?.ToString() ?? "0.0.0.0";
            form.Add(new StringContent(appVersion), "app_version");

            // OS info — combine OSDescription + arch for triage clarity.
            var arch = System.Runtime.InteropServices.RuntimeInformation.ProcessArchitecture;
            var osInfo =
                $"{System.Runtime.InteropServices.RuntimeInformation.OSDescription} ({Environment.OSVersion.Version}) {arch}";
            form.Add(new StringContent(osInfo), "os_info");

            if (!string.IsNullOrWhiteSpace(userEmail))
                form.Add(new StringContent(userEmail), "user_email");

            // Context JSON — backend accepts a stringified JSON blob in the
            // multipart "context" field and parses it server-side.
            var ctx = context != null
                ? new Dictionary<string, object?>(context)
                : new Dictionary<string, object?>();
            ctx["platform"] = "windows";
            ctx["arch"] = arch.ToString();
            ctx["dotnet"] = Environment.Version.ToString();
            ctx["locale"] = System.Globalization.CultureInfo.CurrentCulture.Name;
            ctx["ts"] = DateTime.UtcNow.ToString("o");
            form.Add(new StringContent(JsonSerializer.Serialize(ctx)), "context");

            // Optional screenshot — open the file as a stream so we don't load
            // the whole image into managed memory.
            if (!string.IsNullOrWhiteSpace(screenshotPath) && File.Exists(screenshotPath))
            {
                var info = new FileInfo(screenshotPath);
                if (info.Length > 5 * 1024 * 1024)
                {
                    return new SubmitResult(false,
                        ErrorMessage: "Screenshot exceeds 5 MB limit.");
                }

                screenshotStream = File.OpenRead(screenshotPath);
                var streamContent = new StreamContent(screenshotStream);
                var ext = Path.GetExtension(screenshotPath).ToLowerInvariant();
                var mime = ext switch
                {
                    ".png" => "image/png",
                    ".jpg" or ".jpeg" => "image/jpeg",
                    _ => "application/octet-stream",
                };
                streamContent.Headers.ContentType = new MediaTypeHeaderValue(mime);
                form.Add(streamContent, "screenshot", Path.GetFileName(screenshotPath));
            }

            using var request = new HttpRequestMessage(HttpMethod.Post, $"{baseUrl}/bug-reports")
            {
                Content = form,
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
        finally
        {
            screenshotStream?.Dispose();
        }
    }

    /// <summary>
    /// Minimal description validator used by the dialog (mirrors backend min-length).
    /// </summary>
    public static bool IsDescriptionValid(string? description) =>
        !string.IsNullOrWhiteSpace(description) && description.Trim().Length >= 10;
}
