using System.Text.Json.Serialization;

namespace DraftRightWindows.Models;

// ── Auth ──

public class LoginRequest
{
    [JsonPropertyName("email")]
    public string Email { get; set; } = string.Empty;

    [JsonPropertyName("password")]
    public string Password { get; set; } = string.Empty;
}

public class RegisterRequest
{
    [JsonPropertyName("email")]
    public string Email { get; set; } = string.Empty;

    [JsonPropertyName("password")]
    public string Password { get; set; } = string.Empty;

    [JsonPropertyName("name")]
    public string Name { get; set; } = string.Empty;
}

public class AuthUser
{
    // Backend uses UUID PKs (Postgres @PrimaryGeneratedColumn('uuid')),
    // not integers. This was originally typed as int and crashed login
    // with "The JSON value could not be converted to System.Int32. Path: $.user.id".
    [JsonPropertyName("id")]
    public string Id { get; set; } = string.Empty;

    [JsonPropertyName("email")]
    public string Email { get; set; } = string.Empty;

    [JsonPropertyName("name")]
    public string Name { get; set; } = string.Empty;
}

public class AuthResponse
{
    [JsonPropertyName("access_token")]
    public string AccessToken { get; set; } = string.Empty;

    [JsonPropertyName("refresh_token")]
    public string RefreshToken { get; set; } = string.Empty;

    [JsonPropertyName("user")]
    public AuthUser User { get; set; } = new();
}

// ── Rewrite ──

public class RewriteRequest
{
    [JsonPropertyName("text")]
    public string Text { get; set; } = string.Empty;

    [JsonPropertyName("tone")]
    public string Tone { get; set; } = string.Empty;

    [JsonPropertyName("target_language")]
    [JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public string? TargetLanguage { get; set; }
}

public class RewriteResponse
{
    [JsonPropertyName("rewritten_text")]
    public string RewrittenText { get; set; } = string.Empty;

    [JsonPropertyName("usage_today")]
    public int UsageToday { get; set; }

    [JsonPropertyName("daily_limit")]
    public int DailyLimit { get; set; }

    /// <summary>
    /// Present only when tone == grammar_check. Backend returns a structured
    /// { grammar: { score, issues: [...] } } in addition to (or instead of) a
    /// flat rewritten_text. Rendered by GrammarCheckView.
    /// </summary>
    [JsonPropertyName("grammar")]
    public GrammarResult? Grammar { get; set; }
}

// ── Grammar Check ──
//
// Returned by /rewrite when tone == "grammar_check".
//
// IMPORTANT: do NOT trust the LLM-returned offset/length. The AI is
// unreliable about character positions across multi-line input — it
// counts code points or visible chars in inconsistent ways. To apply a
// fix, search the current text for `Original` near `Offset` and resolve
// the actual range at fix time. See memory feedback_llm_offsets_unreliable.

public class GrammarIssue
{
    /// <summary>"spelling" | "grammar" | "style" | other</summary>
    [JsonPropertyName("type")]
    public string Type { get; set; } = string.Empty;

    [JsonPropertyName("offset")]
    public int Offset { get; set; }

    [JsonPropertyName("length")]
    public int Length { get; set; }

    /// <summary>The actual text the issue is about — anchor for content-resolved replacement.</summary>
    [JsonPropertyName("original")]
    public string Original { get; set; } = string.Empty;

    [JsonPropertyName("suggestion")]
    public string Suggestion { get; set; } = string.Empty;

    [JsonPropertyName("reason")]
    public string Reason { get; set; } = string.Empty;
}

public class GrammarResult
{
    [JsonPropertyName("score")]
    public int Score { get; set; }

    [JsonPropertyName("issues")]
    public List<GrammarIssue> Issues { get; set; } = new();
}

// ── Subscription ──

public class SubscriptionPlan
{
    [JsonPropertyName("name")]
    public string Name { get; set; } = string.Empty;

    [JsonPropertyName("daily_limit")]
    public int DailyLimit { get; set; }
}

public class SubscriptionResponse
{
    [JsonPropertyName("plan")]
    public SubscriptionPlan Plan { get; set; } = new();

    [JsonPropertyName("status")]
    public string Status { get; set; } = string.Empty;

    [JsonPropertyName("usage_today")]
    public int UsageToday { get; set; }
}
