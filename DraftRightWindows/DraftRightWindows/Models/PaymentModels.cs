using System;
using System.Collections.Generic;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace DraftRightWindows.Models;

/// <summary>
/// Identity for one payment method advertised by GET /payment/methods.
/// Mirrors backend <c>PaymentMethod</c> enum + Flutter/macOS clients
/// 1:1 (same wire names).  Adding a method = add a value + extend
/// <see cref="WireName"/> + add a descriptor.
/// </summary>
public enum PaymentMethodKind
{
    LemonSqueezy,
    Stripe,
    VietQr,
    BankTransfer,
    PayPal,
}

public static class PaymentMethodKindExtensions
{
    public static string WireName(this PaymentMethodKind kind) => kind switch
    {
        PaymentMethodKind.LemonSqueezy => "lemonsqueezy",
        PaymentMethodKind.Stripe       => "stripe",
        PaymentMethodKind.VietQr       => "vietqr",
        PaymentMethodKind.BankTransfer => "bank_transfer",
        PaymentMethodKind.PayPal       => "paypal",
        _ => throw new ArgumentOutOfRangeException(nameof(kind)),
    };

    /// <summary>
    /// Returns null for unknown wire names so the catalog gracefully
    /// ignores methods this client doesn't yet implement (forward-
    /// compat with backend additions).
    /// </summary>
    public static PaymentMethodKind? FromWire(string value) => value switch
    {
        "lemonsqueezy"  => PaymentMethodKind.LemonSqueezy,
        "stripe"        => PaymentMethodKind.Stripe,
        "vietqr"        => PaymentMethodKind.VietQr,
        "bank_transfer" => PaymentMethodKind.BankTransfer,
        "paypal"        => PaymentMethodKind.PayPal,
        _               => null,
    };
}

/// <summary>
/// Billing cadence supported by the backend <c>plans.billing_period</c>
/// column.  Mirrors <c>BillingPeriod</c> on Flutter / macOS (1:1, same
/// wire names) so the UI never threads raw <c>"monthly"</c> /
/// <c>"yearly"</c> strings through services.
/// </summary>
public enum BillingPeriod
{
    Monthly,
    Yearly,
}

public static class BillingPeriodExtensions
{
    /// <summary>Lowercase identifier matching <c>plans.billing_period</c>.</summary>
    public static string WireName(this BillingPeriod period) => period switch
    {
        BillingPeriod.Monthly => "monthly",
        BillingPeriod.Yearly  => "yearly",
        _ => throw new ArgumentOutOfRangeException(nameof(period)),
    };

    /// <summary>User-facing English label.</summary>
    public static string DisplayName(this BillingPeriod period) => period switch
    {
        BillingPeriod.Monthly => "Monthly",
        BillingPeriod.Yearly  => "Yearly",
        _ => throw new ArgumentOutOfRangeException(nameof(period)),
    };

    /// <summary>Parse a wire value (case-insensitive).  Null for unknown / Free.</summary>
    public static BillingPeriod? FromWire(string? value)
    {
        if (string.IsNullOrEmpty(value)) return null;
        return value.ToLowerInvariant() switch
        {
            "monthly" => BillingPeriod.Monthly,
            "yearly"  => BillingPeriod.Yearly,
            _         => null,
        };
    }
}

/// <summary>UI metadata for one <see cref="PaymentMethodKind"/>.</summary>
public sealed record PaymentMethodDescriptor(
    PaymentMethodKind Kind,
    string DisplayName,
    string Description)
{
    public static PaymentMethodDescriptor ForKind(PaymentMethodKind kind) => kind switch
    {
        PaymentMethodKind.LemonSqueezy => new(kind,
            "Credit / Debit Card",
            "Visa, Mastercard, Apple Pay (via Lemon Squeezy)"),
        PaymentMethodKind.Stripe => new(kind,
            "Stripe",
            "Credit card via Stripe"),
        PaymentMethodKind.VietQr => new(kind,
            "VietQR (scan to pay)",
            "Scan with any Vietnamese banking app — auto-confirms"),
        PaymentMethodKind.BankTransfer => new(kind,
            "Bank Transfer",
            "Manual transfer with reference code"),
        PaymentMethodKind.PayPal => new(kind,
            "PayPal",
            "Pay with PayPal balance or card"),
        _ => throw new ArgumentOutOfRangeException(nameof(kind)),
    };
}

/// <summary>
/// Typed view of the JSON returned by POST /payment/checkout.
/// Backend <c>CheckoutResult</c> is a union of three shapes — model
/// each as a derived class so UI dispatches on type, not field
/// presence.  Mirrors Flutter's sealed <c>CheckoutResult</c> and the
/// Swift enum.
/// </summary>
public abstract class CheckoutResult
{
    public string ReferenceCode { get; init; } = string.Empty;

    /// <summary>
    /// Decode from a backend envelope.  Field priority: redirect_url
    /// > qr_data > bank_info.  Throws <see cref="InvalidOperationException"/>
    /// when the response has none of the three.
    /// </summary>
    public static CheckoutResult FromJson(JsonElement root)
    {
        string ReferenceCodeOf()
        {
            if (root.TryGetProperty("payment", out var p)
                && p.TryGetProperty("reference_code", out var pref)
                && pref.ValueKind == JsonValueKind.String)
                return pref.GetString() ?? string.Empty;
            if (root.TryGetProperty("reference_code", out var tref)
                && tref.ValueKind == JsonValueKind.String)
                return tref.GetString() ?? string.Empty;
            return string.Empty;
        }
        var refCode = ReferenceCodeOf();

        if (root.TryGetProperty("redirect_url", out var redirect)
            && redirect.ValueKind == JsonValueKind.String
            && !string.IsNullOrEmpty(redirect.GetString()))
        {
            return new RedirectCheckout
            {
                ReferenceCode = refCode,
                Url = redirect.GetString() ?? string.Empty,
            };
        }

        BankInfo? bank = null;
        if (root.TryGetProperty("bank_info", out var bankRaw)
            && bankRaw.ValueKind == JsonValueKind.Object)
        {
            bank = BankInfo.FromJson(bankRaw);
        }

        if (root.TryGetProperty("qr_data", out var qr)
            && qr.ValueKind == JsonValueKind.String
            && !string.IsNullOrEmpty(qr.GetString()))
        {
            return new QrCheckout
            {
                ReferenceCode = refCode,
                ImageUrl = qr.GetString() ?? string.Empty,
                BankInfo = bank,
            };
        }

        if (bank is not null)
        {
            return new BankTransferCheckout
            {
                ReferenceCode = refCode,
                Info = bank,
            };
        }

        throw new InvalidOperationException(
            "Backend returned a checkout response with none of redirect_url / qr_data / bank_info");
    }
}

public sealed class RedirectCheckout : CheckoutResult
{
    public string Url { get; init; } = string.Empty;
}

public sealed class QrCheckout : CheckoutResult
{
    public string ImageUrl { get; init; } = string.Empty;
    public BankInfo? BankInfo { get; init; }
}

public sealed class BankTransferCheckout : CheckoutResult
{
    public BankInfo Info { get; init; } = new();
}

public sealed class BankInfo
{
    public string BankName { get; init; } = string.Empty;
    public string AccountNumber { get; init; } = string.Empty;
    public string AccountName { get; init; } = string.Empty;
    public double Amount { get; init; }
    public string Currency { get; init; } = "VND";
    public string Reference { get; init; } = string.Empty;

    public static BankInfo FromJson(JsonElement j) => new()
    {
        BankName      = j.TryGetProperty("bank_name", out var bn)      ? bn.GetString() ?? "" : "",
        AccountNumber = j.TryGetProperty("account_number", out var an) ? an.GetString() ?? "" : "",
        AccountName   = j.TryGetProperty("account_name", out var nm)   ? nm.GetString() ?? "" : "",
        Amount        = j.TryGetProperty("amount", out var am)         ? am.GetDouble()     : 0,
        Currency      = j.TryGetProperty("currency", out var cu)       ? cu.GetString() ?? "VND" : "VND",
        Reference     = j.TryGetProperty("reference", out var rf)      ? rf.GetString() ?? "" : "",
    };
}

/// <summary>
/// Lifecycle of a single payment.  Mirrors backend PaymentStatus +
/// synthetic <c>NotFound</c> for /payment/status/:ref's "not_found".
/// </summary>
public enum PaymentStatus
{
    Pending,
    Completed,
    Failed,
    Expired,
    Refunded,
    NotFound,
    Unknown,
}

public static class PaymentStatusExtensions
{
    public static PaymentStatus FromWire(string value) => value switch
    {
        "pending"   => PaymentStatus.Pending,
        "completed" => PaymentStatus.Completed,
        "failed"    => PaymentStatus.Failed,
        "expired"   => PaymentStatus.Expired,
        "refunded"  => PaymentStatus.Refunded,
        "not_found" => PaymentStatus.NotFound,
        _           => PaymentStatus.Unknown,
    };

    public static bool IsTerminal(this PaymentStatus s) => s is
        PaymentStatus.Completed or
        PaymentStatus.Failed or
        PaymentStatus.Expired or
        PaymentStatus.Refunded;

    public static bool IsSuccess(this PaymentStatus s) => s == PaymentStatus.Completed;
}

public sealed class PaymentStatusUpdate
{
    public string ReferenceCode { get; init; } = string.Empty;
    public PaymentStatus Status { get; init; } = PaymentStatus.Pending;
    public double? Amount { get; init; }
    public string? Currency { get; init; }
    public string? PlanName { get; init; }
}

/// <summary>Raw JSON envelopes the API returns; deserialised by ApiClient.</summary>
public sealed class PaymentMethodsResponse
{
    [JsonPropertyName("methods")]
    public List<string> Methods { get; set; } = new();
}

public sealed class CustomerPortalResponse
{
    [JsonPropertyName("url")]
    public string Url { get; set; } = string.Empty;
}

public sealed class PaymentStatusResponse
{
    [JsonPropertyName("status")]        public string Status { get; set; } = string.Empty;
    [JsonPropertyName("method")]        public string? Method { get; set; }
    [JsonPropertyName("amount")]        public double? Amount { get; set; }
    [JsonPropertyName("currency")]      public string? Currency { get; set; }
    [JsonPropertyName("reference_code")] public string ReferenceCode { get; set; } = string.Empty;
    [JsonPropertyName("plan_name")]     public string? PlanName { get; set; }

    public PaymentStatusUpdate ToUpdate() => new()
    {
        ReferenceCode = ReferenceCode,
        Status        = PaymentStatusExtensions.FromWire(Status),
        Amount        = Amount,
        Currency      = Currency,
        PlanName      = PlanName,
    };
}

public sealed class PlanRow
{
    [JsonPropertyName("id")]              public string Id { get; set; } = string.Empty;
    [JsonPropertyName("name")]            public string Name { get; set; } = string.Empty;
    [JsonPropertyName("billing_period")]  public string BillingPeriod { get; set; } = string.Empty;
    [JsonPropertyName("is_active")]       public bool IsActive { get; set; } = true;
    /// <summary>USD for LS/Stripe/PayPal plans; VND for VietQR/bank-transfer.</summary>
    [JsonPropertyName("currency")]        public string Currency { get; set; } = "USD";
}
