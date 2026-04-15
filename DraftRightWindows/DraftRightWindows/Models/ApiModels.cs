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
    [JsonPropertyName("id")]
    public int Id { get; set; }

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
