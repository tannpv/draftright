using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Net;
using System.Net.Http;
using System.Net.Sockets;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;

namespace DraftRightWindows.Services;

/// <summary>
/// Google OAuth 2.0 for an installed Windows app — loopback redirect + PKCE.
///
/// Per Google's "Installed app" guidance, this is a *public* client: the
/// client_secret ships with the binary and is not security-critical; PKCE is
/// the actual proof-of-possession. Custom URL schemes (used by the macOS/iOS
/// clients) aren't needed on Windows — Google supports listening on
/// <c>http://127.0.0.1:&lt;ephemeral-port&gt;</c> instead, which avoids registering
/// a protocol handler in the registry.
///
/// Flow:
///   1. Generate PKCE verifier + S256 challenge.
///   2. Open an HttpListener on a free loopback port.
///   3. Launch the system browser at the Google consent screen.
///   4. Google redirects back to <c>http://127.0.0.1:PORT/?code=...</c>; we
///      capture the code, respond with a friendly HTML page so the tab
///      doesn't look broken.
///   5. Exchange the code (with the verifier) for an <c>id_token</c>.
///
/// The returned id_token is passed to the backend's <c>/auth/social</c>
/// endpoint, which verifies it via Google's tokeninfo and creates/links
/// the user.
/// </summary>
public static class GoogleOAuth
{
    // Desktop-type OAuth client (project 22951518033). Public client: the
    // "secret" is distributed with every Windows copy of the app — not a real
    // secret. PKCE is the actual proof-of-possession. Kept out of git for
    // hygiene (see GoogleOAuthConfig.cs.template).
    private const string ClientId     = "22951518033-oq7okrvvbb26eqsb7c0avsb1ic165ole.apps.googleusercontent.com";
    private static string ClientSecret => GoogleOAuthConfig.ClientSecret;
    private const string AuthEndpoint  = "https://accounts.google.com/o/oauth2/v2/auth";
    private const string TokenEndpoint = "https://oauth2.googleapis.com/token";
    private const string Scopes        = "openid email profile";

    /// <summary>Run the full flow → return Google id_token. Throws on cancel or error.</summary>
    public static async Task<string> AuthenticateAsync()
    {
        var verifier  = PkceVerifier();
        var challenge = PkceChallenge(verifier);
        var state     = RandomToken(16);

        var port        = FindFreeLoopbackPort();
        var redirectUri = $"http://127.0.0.1:{port}/";

        using var listener = new HttpListener();
        listener.Prefixes.Add(redirectUri);
        listener.Start();

        var authUrl = AuthEndpoint
            + "?response_type=code"
            + "&client_id="              + Uri.EscapeDataString(ClientId)
            + "&redirect_uri="           + Uri.EscapeDataString(redirectUri)
            + "&scope="                  + Uri.EscapeDataString(Scopes)
            + "&code_challenge="         + challenge
            + "&code_challenge_method=S256"
            + "&state="                  + state;

        Process.Start(new ProcessStartInfo { FileName = authUrl, UseShellExecute = true });

        // Wait for Google to redirect back. listener.GetContextAsync blocks
        // until the browser hits the loopback — no timeout here because the
        // user may take their time on the consent screen; the caller decides
        // whether to surface a cancel UI.
        var ctx = await listener.GetContextAsync();
        var query = ParseQuery(ctx.Request.Url!.Query);

        // Always show the user a clean "you can close this" page, even on
        // error — a blank browser tab looks worse than the failure itself.
        WriteHtmlResponse(ctx,
            "<html><body style='font:14px sans-serif;text-align:center;padding:40px'>"
            + "<h2>Sign-in complete</h2>"
            + "<p>You can close this window and return to DraftRight.</p>"
            + "</body></html>");

        if (query.TryGetValue("error", out var err) && !string.IsNullOrEmpty(err))
            throw new InvalidOperationException("Google sign-in error: " + err);
        if (!query.TryGetValue("code", out var code) || string.IsNullOrEmpty(code))
            throw new InvalidOperationException("Google sign-in returned no code");
        if (!query.TryGetValue("state", out var gotState) || gotState != state)
            throw new InvalidOperationException("OAuth state mismatch (possible CSRF)");

        return await ExchangeCodeForIdTokenAsync(code, verifier, redirectUri);
    }

    private static async Task<string> ExchangeCodeForIdTokenAsync(string code, string verifier, string redirectUri)
    {
        using var http = new HttpClient();
        var form = new FormUrlEncodedContent(new Dictionary<string, string>
        {
            ["code"]          = code,
            ["client_id"]     = ClientId,
            ["client_secret"] = ClientSecret,
            ["redirect_uri"]  = redirectUri,
            ["grant_type"]    = "authorization_code",
            ["code_verifier"] = verifier,
        });

        var resp = await http.PostAsync(TokenEndpoint, form);
        var json = await resp.Content.ReadAsStringAsync();
        if (!resp.IsSuccessStatusCode)
            throw new InvalidOperationException($"Google token exchange failed: {(int)resp.StatusCode} {json}");

        using var doc = JsonDocument.Parse(json);
        if (!doc.RootElement.TryGetProperty("id_token", out var idTok))
            throw new InvalidOperationException("Token response had no id_token");
        var s = idTok.GetString();
        if (string.IsNullOrEmpty(s)) throw new InvalidOperationException("Empty id_token in token response");
        return s;
    }

    // --- helpers ---

    /// <summary>Bind a TcpListener to port 0 → OS picks a free port → release.</summary>
    private static int FindFreeLoopbackPort()
    {
        var l = new TcpListener(IPAddress.Loopback, 0);
        l.Start();
        try { return ((IPEndPoint)l.LocalEndpoint).Port; }
        finally { l.Stop(); }
    }

    private static string PkceVerifier()
    {
        var bytes = new byte[32];
        RandomNumberGenerator.Fill(bytes);
        return Base64Url(bytes);
    }

    private static string PkceChallenge(string verifier)
    {
        var hash = SHA256.HashData(Encoding.UTF8.GetBytes(verifier));
        return Base64Url(hash);
    }

    private static string RandomToken(int byteLen)
    {
        var bytes = new byte[byteLen];
        RandomNumberGenerator.Fill(bytes);
        return Base64Url(bytes);
    }

    /// <summary>base64url without padding (the encoding OAuth/PKCE require).</summary>
    private static string Base64Url(byte[] data) =>
        Convert.ToBase64String(data).TrimEnd('=').Replace('+', '-').Replace('/', '_');

    private static Dictionary<string, string> ParseQuery(string query)
    {
        var dict = new Dictionary<string, string>(StringComparer.Ordinal);
        if (string.IsNullOrEmpty(query)) return dict;
        if (query[0] == '?') query = query.Substring(1);
        foreach (var pair in query.Split('&'))
        {
            if (string.IsNullOrEmpty(pair)) continue;
            var eq = pair.IndexOf('=');
            if (eq < 0) { dict[Uri.UnescapeDataString(pair)] = ""; continue; }
            var key = Uri.UnescapeDataString(pair.Substring(0, eq));
            var val = Uri.UnescapeDataString(pair.Substring(eq + 1));
            dict[key] = val;
        }
        return dict;
    }

    private static void WriteHtmlResponse(HttpListenerContext ctx, string html)
    {
        var bytes = Encoding.UTF8.GetBytes(html);
        ctx.Response.ContentType = "text/html; charset=utf-8";
        ctx.Response.ContentLength64 = bytes.Length;
        ctx.Response.OutputStream.Write(bytes, 0, bytes.Length);
        ctx.Response.OutputStream.Close();
    }
}
