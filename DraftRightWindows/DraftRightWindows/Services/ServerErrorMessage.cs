// Pure error-body parsing — no HttpClient, no UI, no Windows dependencies, so
// the headless test project can compile this file directly. See issue #80 and
// the header of UpdateManifest.cs for why that matters.
using System.Collections.Generic;
using System.Text.Json;

namespace DraftRightWindows.Services;

/// <summary>
/// Turns an error response body into the one-line message the UI shows.
/// </summary>
public static class ServerErrorMessage
{
    private const string MessageField = "message";
    private const string ErrorField = "error";

    /// <summary>
    /// Extracts a user-facing message from an error response body, handling
    /// every shape our backends emit:
    ///   • NestJS class-validator: {"message":["email must be an email"]} (string[])
    ///   • NestJS other errors:    {"message":"…"} (string)
    ///   • Go backend:             {"error":"Account disabled","code":"…"} (string)
    /// Without the Go `error` case, its bodies fell through to the raw JSON and
    /// users saw stack-trace soup (BUG-44: "Account disabled" leaked as JSON
    /// after the Go cutover). Falls back to the raw body when unparseable.
    /// See BUG-18 / BUG-19 (2026-05-29) for the original NestJS handling.
    /// </summary>
    public static string Extract(string body)
    {
        if (string.IsNullOrWhiteSpace(body)) return body;
        try
        {
            using var doc = JsonDocument.Parse(body);
            var root = doc.RootElement;
            if (root.ValueKind != JsonValueKind.Object) return body;

            if (root.TryGetProperty(MessageField, out var msg))
            {
                if (msg.ValueKind == JsonValueKind.String)
                {
                    var s = msg.GetString();
                    if (!string.IsNullOrEmpty(s)) return s;
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
                    if (parts.Count > 0) return string.Join("; ", parts);
                }
            }

            // Go backend error shape: {"error":"…","code":"…","request_id":"…"}.
            if (root.TryGetProperty(ErrorField, out var err)
                && err.ValueKind == JsonValueKind.String)
            {
                var s = err.GetString();
                if (!string.IsNullOrEmpty(s)) return s;
            }
        }
        catch
        {
            /* body wasn't JSON — keep raw */
        }
        return body;
    }
}
