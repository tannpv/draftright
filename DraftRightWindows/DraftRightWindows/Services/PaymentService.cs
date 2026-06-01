using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using DraftRightWindows.Helpers;
using DraftRightWindows.Models;

namespace DraftRightWindows.Services;

/// <summary>
/// Orchestrates the upgrade flow + customer-portal launch on Windows.
///
/// <para>
/// Owns the {kind → handler} map (strategy pattern matching Flutter
/// + macOS) and produces <see cref="IObservable{PaymentStatusUpdate}"/>
/// streams for the foreground poller used by the QR / bank dialogs.
/// </para>
/// </summary>
public sealed class PaymentService
{
    public ApiClient Api { get; }
    private readonly Dictionary<PaymentMethodKind, IPaymentHandler> _handlers = new();

    public PaymentService(ApiClient api)
    {
        Api = api;
        RegisterDefaultHandlers();
    }

    private void RegisterDefaultHandlers()
    {
        // Wrap in a lambda because `WatchPayment`'s optional
        // `interval`/`timeout` parameters block method-group
        // conversion to `Func<string, ...>` under the Windows CI
        // toolchain (CS0123).  Pre-existing breakage from the
        // 2026-05-31 windows-payment-method-picker commit.
        Func<string, IObservable<PaymentStatusUpdate>> watch = refCode => WatchPayment(refCode);
        Register(new RedirectPaymentHandler(PaymentMethodKind.LemonSqueezy));
        Register(new RedirectPaymentHandler(PaymentMethodKind.Stripe));
        Register(new RedirectPaymentHandler(PaymentMethodKind.PayPal));
        Register(new QrPaymentHandler(watch));
        Register(new BankTransferPaymentHandler(watch));
    }

    /// <summary>Override the handler for one kind (tests + future overrides).</summary>
    public void Register(IPaymentHandler handler) => _handlers[handler.Kind] = handler;

    // MARK: Public API

    /// <summary>
    /// Methods the user can pick from.  No Apple-store policy gate
    /// on Windows; show everything the backend enables.
    /// </summary>
    public Task<List<PaymentMethodKind>> ListAvailableMethodsAsync()
        => Api.ListPaymentMethodsAsync();

    /// <summary>
    /// Resolve the Pro-tier plan id from /plans for the requested
    /// method + billing cadence.
    ///
    ///   - Currency-aware so VietQR doesn't pick a USD plan (the QR
    ///     would bake "$4.99 đồng" — useless).
    ///   - Cadence-aware so the Monthly / Yearly toggle on the
    ///     Subscription tab charges the matching variant.
    ///
    /// Mirrors <c>resolveProPlanId</c> on Flutter / macOS.
    /// </summary>
    public async Task<string> ResolveProPlanIdAsync(
        PaymentMethodKind? method = null,
        BillingPeriod? billingPeriod = null)
    {
        var plans = await Api.ListPlansAsync();
        var currency = method.HasValue ? CurrencyFor(method.Value) : null;
        var paid = plans
            .Where(p => !string.IsNullOrEmpty(p.BillingPeriod)
                        && !string.Equals(p.BillingPeriod, "none", StringComparison.OrdinalIgnoreCase)
                        && p.IsActive
                        && (currency == null
                            || string.Equals(p.Currency, currency, StringComparison.OrdinalIgnoreCase)))
            .ToList();
        if (paid.Count == 0)
            throw new InvalidOperationException(
                currency != null
                    ? $"Could not find a Pro plan in {currency} for {method}"
                    : "Could not find a Pro plan in the catalog");
        if (billingPeriod.HasValue)
        {
            var exact = paid.FirstOrDefault(p =>
                BillingPeriodExtensions.FromWire(p.BillingPeriod) == billingPeriod.Value);
            if (exact != null && !string.IsNullOrEmpty(exact.Id))
                return exact.Id;
        }
        // No exact cadence match (or none requested) — fall back to
        // monthly, then the first paid plan.
        var monthly = paid.FirstOrDefault(p =>
            BillingPeriodExtensions.FromWire(p.BillingPeriod) == BillingPeriod.Monthly)
            ?? paid[0];
        if (string.IsNullOrEmpty(monthly.Id))
            throw new InvalidOperationException("Pro plan row is missing an id");
        return monthly.Id;
    }

    /// <summary>
    /// Currency the strategy expects to charge the plan in.  VietQR +
    /// bank-transfer settle in VND (Vietnamese-bank-only spec); all
    /// others default to USD.  Mirrors <c>_currencyFor</c> on Flutter.
    /// </summary>
    public static string CurrencyFor(PaymentMethodKind method) => method switch
    {
        PaymentMethodKind.VietQr or PaymentMethodKind.BankTransfer => "VND",
        _ => "USD",
    };

    public async Task UpgradeAsync(PaymentMethodKind method, string planId, IPaymentSheetPresenter presenter)
    {
        if (!_handlers.TryGetValue(method, out var handler))
        {
            throw new InvalidOperationException($"No handler registered for {method}");
        }
        var result = await Api.CreateCheckoutAsync(planId, method);
        DRLogger.Log(
            $"checkout created: method={method.WireName()} ref={result.ReferenceCode}",
            DRLogger.Category.API);
        await handler.HandleAsync(result, presenter);
    }

    public async Task OpenCustomerPortalAsync()
    {
        var url = await Api.GetCustomerPortalUrlAsync();
        Process.Start(new ProcessStartInfo
        {
            FileName = url,
            UseShellExecute = true,
        });
    }

    // MARK: Foreground status poller

    /// <summary>
    /// Poll /payment/status/:ref until terminal, the deadline
    /// elapses, or the consumer drops the subscription.  Lightweight
    /// <see cref="IObservable{T}"/> wrapper (no Rx dep) — handles
    /// fire-and-forget by the host dialog.
    /// </summary>
    public IObservable<PaymentStatusUpdate> WatchPayment(
        string referenceCode,
        TimeSpan? interval = null,
        TimeSpan? timeout = null)
    {
        return new PaymentStatusObservable(
            Api,
            referenceCode,
            interval ?? TimeSpan.FromSeconds(3),
            timeout ?? TimeSpan.FromMinutes(15));
    }
}

/// <summary>
/// Tiny IObservable implementation that runs the poll loop on a
/// background task and yields a synthetic <c>Expired</c> if the
/// deadline passes before a terminal status arrives.
/// </summary>
internal sealed class PaymentStatusObservable : IObservable<PaymentStatusUpdate>
{
    private readonly ApiClient _api;
    private readonly string _ref;
    private readonly TimeSpan _interval;
    private readonly TimeSpan _timeout;

    public PaymentStatusObservable(ApiClient api, string referenceCode, TimeSpan interval, TimeSpan timeout)
    {
        _api = api;
        _ref = referenceCode;
        _interval = interval;
        _timeout = timeout;
    }

    public IDisposable Subscribe(IObserver<PaymentStatusUpdate> observer)
    {
        var cts = new CancellationTokenSource();
        _ = Task.Run(async () =>
        {
            var deadline = DateTime.UtcNow + _timeout;
            while (!cts.IsCancellationRequested && DateTime.UtcNow < deadline)
            {
                try
                {
                    var update = await _api.GetPaymentStatusAsync(_ref);
                    if (cts.IsCancellationRequested) return;
                    observer.OnNext(update);
                    if (update.Status.IsTerminal())
                    {
                        observer.OnCompleted();
                        return;
                    }
                }
                catch (Exception e)
                {
                    DRLogger.Warn($"status poll failed: {e.Message}", DRLogger.Category.API);
                }
                try { await Task.Delay(_interval, cts.Token); }
                catch (TaskCanceledException) { return; }
            }
            if (!cts.IsCancellationRequested)
            {
                observer.OnNext(new PaymentStatusUpdate
                {
                    ReferenceCode = _ref,
                    Status = PaymentStatus.Expired,
                });
                observer.OnCompleted();
            }
        }, cts.Token);
        return new Subscription(cts);
    }

    private sealed class Subscription : IDisposable
    {
        private readonly CancellationTokenSource _cts;
        public Subscription(CancellationTokenSource cts) { _cts = cts; }
        public void Dispose() { _cts.Cancel(); _cts.Dispose(); }
    }
}
