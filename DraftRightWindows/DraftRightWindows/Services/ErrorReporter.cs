using System;
using System.Collections.Generic;
using System.IO;
using System.Net.Http;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;

namespace DraftRightWindows.Services;

/// <summary>
/// Sends unhandled exceptions to the DraftRight backend's /errors
/// endpoint. Pairs with App.cs's existing local crash logging — every
/// crash gets BOTH a local file and a server report.
///
/// Privacy: never sends user-typed text. Only error type, stack, and
/// a small context (OS version, app version, locale).
/// </summary>
public static class ErrorReporter
{
    private static readonly HttpClient _http = new() { Timeout = TimeSpan.FromSeconds(10) };
    private static string _backendUrl = "https://api.draftright.info";
    private static Func<string?>? _bearerTokenProvider;
    private static readonly string _queuePath = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "DraftRight", "error_queue.json");

    public static void Configure(string backendUrl, Func<string?>? bearerTokenProvider)
    {
        _backendUrl = backendUrl.TrimEnd('/');
        _bearerTokenProvider = bearerTokenProvider;

        // Flush anything stranded from a previous run
        _ = Task.Run(FlushPersistedQueue);
    }

    /// <summary>
    /// Fire-and-forget report. Falls back to a local queue if the network
    /// is down; the queue is drained on next startup.
    /// </summary>
    public static void Report(Exception ex, string source = "unknown", string severity = "error")
    {
        try
        {
            var report = Build(ex, source, severity);
            _ = SendOrQueue(report);
        }
        catch
        {
            // Reporting must never throw on the calling thread. Swallow.
        }
    }

    public static void ReportHandled(Exception ex, string severity = "warning")
    {
        Report(ex, source: "handled", severity: severity);
    }

    private static Dictionary<string, object?> Build(Exception ex, string source, string severity)
    {
        var asm = System.Reflection.Assembly.GetExecutingAssembly();
        var version = asm.GetName().Version?.ToString() ?? "?";

        return new Dictionary<string, object?>
        {
            ["platform"] = "windows",
            ["app_version"] = version,
            ["severity"] = severity,
            ["error_type"] = Truncate(ex.GetType().FullName ?? ex.GetType().Name, 200),
            ["message"] = Truncate(ex.Message ?? "", 5000),
            ["stack_trace"] = Truncate(ex.StackTrace ?? "", 20000),
            ["context"] = new Dictionary<string, object?>
            {
                ["source"] = source,
                ["os"] = Environment.OSVersion.VersionString,
                ["arch"] = System.Runtime.InteropServices.RuntimeInformation.ProcessArchitecture.ToString(),
                ["dotnet"] = Environment.Version.ToString(),
                ["locale"] = System.Globalization.CultureInfo.CurrentCulture.Name,
                ["ts"] = DateTime.UtcNow.ToString("o"),
            },
        };
    }

    private static string Truncate(string s, int max) =>
        s.Length > max ? s.Substring(0, max) : s;

    private static async Task SendOrQueue(Dictionary<string, object?> report)
    {
        var json = JsonSerializer.Serialize(report);
        try
        {
            using var req = new HttpRequestMessage(HttpMethod.Post, $"{_backendUrl}/errors");
            req.Content = new StringContent(json, Encoding.UTF8, "application/json");

            var token = _bearerTokenProvider?.Invoke();
            if (!string.IsNullOrEmpty(token))
                req.Headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", token);

            var resp = await _http.SendAsync(req).ConfigureAwait(false);
            if (resp.IsSuccessStatusCode) return;
            // Server rejected → drop. Don't queue malformed reports forever.
        }
        catch
        {
            // Network failed — persist for next launch
            PersistToQueue(json);
        }
    }

    private static void PersistToQueue(string json)
    {
        try
        {
            Directory.CreateDirectory(Path.GetDirectoryName(_queuePath)!);
            var existing = File.Exists(_queuePath)
                ? File.ReadAllLines(_queuePath)
                : Array.Empty<string>();
            var trimmed = existing.Length > 100 ? existing[^100..] : existing;
            File.WriteAllLines(_queuePath, [..trimmed, json]);
        }
        catch
        {
            // Persistence is best-effort
        }
    }

    private static async Task FlushPersistedQueue()
    {
        if (!File.Exists(_queuePath)) return;
        try
        {
            var lines = File.ReadAllLines(_queuePath);
            File.Delete(_queuePath); // optimistic — we'll re-queue failures
            var remaining = new List<string>();

            foreach (var line in lines)
            {
                try
                {
                    using var req = new HttpRequestMessage(HttpMethod.Post, $"{_backendUrl}/errors");
                    req.Content = new StringContent(line, Encoding.UTF8, "application/json");

                    var token = _bearerTokenProvider?.Invoke();
                    if (!string.IsNullOrEmpty(token))
                        req.Headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", token);

                    var resp = await _http.SendAsync(req).ConfigureAwait(false);
                    if (!resp.IsSuccessStatusCode) remaining.Add(line);
                }
                catch
                {
                    remaining.Add(line);
                }
            }

            if (remaining.Count > 0)
            {
                Directory.CreateDirectory(Path.GetDirectoryName(_queuePath)!);
                File.WriteAllLines(_queuePath, remaining);
            }
        }
        catch
        {
            // ignore
        }
    }
}
