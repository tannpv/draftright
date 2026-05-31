using System;
using System.Diagnostics;
using System.Threading.Tasks;
using DraftRightWindows.Models;
using DraftRightWindows.Views;

namespace DraftRightWindows.Services;

/// <summary>
/// Post-checkout UX for one <see cref="PaymentMethodKind"/>.
///
/// <para>
/// Mirrors the strategy pattern shipped on Flutter + macOS.  Adding a
/// new method = new class implementing this + register in
/// <see cref="PaymentService"/>'s handler map.  Subscription UI never
/// branches on payment kind.
/// </para>
/// </summary>
public interface IPaymentHandler
{
    PaymentMethodKind Kind { get; }

    /// <summary>
    /// Drive the post-checkout flow.  Some handlers shell out to the
    /// browser (no UI state), others present a sheet via the
    /// <paramref name="presenter"/>.
    /// </summary>
    Task HandleAsync(CheckoutResult result, IPaymentSheetPresenter presenter);
}

/// <summary>
/// Lightweight binding that lets handlers ask the host form to
/// present a checkout dialog without depending on dialog types
/// directly.  Implemented by <see cref="SubscriptionTabBuilder"/>'s
/// owning form.
/// </summary>
public interface IPaymentSheetPresenter
{
    void PresentQrDialog(QrCheckout checkout, IObservable<PaymentStatusUpdate>? statusStream);
    void PresentBankTransferDialog(BankTransferCheckout checkout, IObservable<PaymentStatusUpdate>? statusStream);
}

/// <summary>
/// Opens the redirect URL in the user's default browser via
/// <see cref="Process.Start"/>.  Used by every URL-based provider:
/// Lemon Squeezy, Stripe, PayPal.  Windows + Direct-Distribution
/// builds carry no IAP rule, so all redirect handlers are enabled.
/// </summary>
public sealed class RedirectPaymentHandler : IPaymentHandler
{
    public PaymentMethodKind Kind { get; }

    public RedirectPaymentHandler(PaymentMethodKind kind)
    {
        Kind = kind;
    }

    public Task HandleAsync(CheckoutResult result, IPaymentSheetPresenter presenter)
    {
        if (result is not RedirectCheckout redirect)
        {
            throw new InvalidOperationException(
                $"RedirectPaymentHandler received non-redirect result: {result.GetType().Name}");
        }
        Process.Start(new ProcessStartInfo
        {
            FileName = redirect.Url,
            UseShellExecute = true,
        });
        return Task.CompletedTask;
    }
}

/// <summary>
/// Presents the VietQR dialog via the host's
/// <see cref="IPaymentSheetPresenter"/>.
/// </summary>
public sealed class QrPaymentHandler : IPaymentHandler
{
    public PaymentMethodKind Kind => PaymentMethodKind.VietQr;

    private readonly Func<string, IObservable<PaymentStatusUpdate>> _statusWatcher;

    public QrPaymentHandler(Func<string, IObservable<PaymentStatusUpdate>> statusWatcher)
    {
        _statusWatcher = statusWatcher;
    }

    public Task HandleAsync(CheckoutResult result, IPaymentSheetPresenter presenter)
    {
        if (result is not QrCheckout qr)
        {
            throw new InvalidOperationException(
                $"QrPaymentHandler received non-qr result: {result.GetType().Name}");
        }
        presenter.PresentQrDialog(qr, _statusWatcher(qr.ReferenceCode));
        return Task.CompletedTask;
    }
}

/// <summary>
/// Presents the bank-transfer dialog via the host's
/// <see cref="IPaymentSheetPresenter"/>.
/// </summary>
public sealed class BankTransferPaymentHandler : IPaymentHandler
{
    public PaymentMethodKind Kind => PaymentMethodKind.BankTransfer;

    private readonly Func<string, IObservable<PaymentStatusUpdate>> _statusWatcher;

    public BankTransferPaymentHandler(Func<string, IObservable<PaymentStatusUpdate>> statusWatcher)
    {
        _statusWatcher = statusWatcher;
    }

    public Task HandleAsync(CheckoutResult result, IPaymentSheetPresenter presenter)
    {
        if (result is not BankTransferCheckout bank)
        {
            throw new InvalidOperationException(
                $"BankTransferPaymentHandler received non-bank result: {result.GetType().Name}");
        }
        presenter.PresentBankTransferDialog(bank, _statusWatcher(bank.ReferenceCode));
        return Task.CompletedTask;
    }
}
