using System;
using System.Drawing;
using DraftRightWindows.Models;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows.Views;

/// <summary>
/// Compact live-status banner used inside <see cref="QrCheckoutDialog"/>
/// and <see cref="BankTransferDialog"/>.  Subscribes to a
/// <see cref="IObservable{PaymentStatusUpdate}"/> (the
/// <c>PaymentService.WatchPayment</c> poller) and:
///   - renders "Waiting for payment…" while pending,
///   - flips to a green tick + "Payment confirmed!" on success,
///   - then auto-closes the parent form via <paramref name="onConfirmed"/>
///     after <see cref="AutoDismissDelay"/>.
/// </summary>
public sealed class PaymentStatusBanner : WinForms.Panel, IObserver<PaymentStatusUpdate>
{
    public TimeSpan AutoDismissDelay { get; set; } = TimeSpan.FromSeconds(2);

    private readonly WinForms.Label _icon;
    private readonly WinForms.Label _text;
    private readonly WinForms.Timer _autoDismiss;
    private readonly Action _onConfirmed;
    private IDisposable? _subscription;

    public PaymentStatusBanner(IObservable<PaymentStatusUpdate>? stream, Action onConfirmed)
    {
        _onConfirmed = onConfirmed;
        BorderStyle = WinForms.BorderStyle.None;
        Height = 36;
        Dock = WinForms.DockStyle.Top;
        Padding = new WinForms.Padding(10, 6, 10, 6);

        _icon = new WinForms.Label
        {
            Text = "⏳",
            AutoSize = true,
            Location = new Point(8, 8),
            Font = new Font("Segoe UI Emoji", 11),
        };
        _text = new WinForms.Label
        {
            Text = "Waiting for payment…",
            AutoSize = true,
            Location = new Point(36, 9),
            Font = new Font("Segoe UI", 9, FontStyle.Bold),
        };
        Controls.Add(_icon);
        Controls.Add(_text);

        _autoDismiss = new WinForms.Timer { Interval = (int)AutoDismissDelay.TotalMilliseconds };
        _autoDismiss.Tick += (_, _) =>
        {
            _autoDismiss.Stop();
            _onConfirmed();
        };

        if (stream is null)
        {
            Visible = false;
        }
        else
        {
            ApplyPending();
            _subscription = stream.Subscribe(this);
        }
    }

    protected override void Dispose(bool disposing)
    {
        if (disposing)
        {
            _subscription?.Dispose();
            _autoDismiss.Dispose();
        }
        base.Dispose(disposing);
    }

    public void OnNext(PaymentStatusUpdate u)
    {
        if (IsDisposed) return;
        if (InvokeRequired)
        {
            try { BeginInvoke(new Action(() => OnNext(u))); } catch { /* form closing */ }
            return;
        }
        ApplyStatus(u.Status);
        if (u.Status.IsSuccess())
        {
            _autoDismiss.Stop();
            _autoDismiss.Interval = Math.Max(50, (int)AutoDismissDelay.TotalMilliseconds);
            _autoDismiss.Start();
        }
    }

    public void OnCompleted() { }

    public void OnError(Exception error) { }

    private void ApplyPending() => ApplyStatus(PaymentStatus.Pending);

    private void ApplyStatus(PaymentStatus status)
    {
        switch (status)
        {
            case PaymentStatus.Pending:
            case PaymentStatus.NotFound:
            case PaymentStatus.Unknown:
                BackColor = Color.FromArgb(30, 64, 175);   // blue 700
                ForeColor = Color.White;
                _icon.Text = "⏳"; _icon.ForeColor = Color.White;
                _text.Text = "Waiting for payment…"; _text.ForeColor = Color.White;
                break;
            case PaymentStatus.Completed:
                BackColor = Color.FromArgb(22, 101, 52);    // green 800
                _icon.Text = "✅"; _icon.ForeColor = Color.White;
                _text.Text = "Payment confirmed!"; _text.ForeColor = Color.White;
                break;
            case PaymentStatus.Failed:
                BackColor = Color.FromArgb(153, 27, 27);    // red 800
                _icon.Text = "❌"; _icon.ForeColor = Color.White;
                _text.Text = "Payment failed. Please try again."; _text.ForeColor = Color.White;
                break;
            case PaymentStatus.Expired:
                BackColor = Color.FromArgb(154, 52, 18);    // orange 800
                _icon.Text = "⌛"; _icon.ForeColor = Color.White;
                _text.Text = "Took too long. If you already paid, check Subscription in a minute.";
                _text.ForeColor = Color.White;
                break;
            case PaymentStatus.Refunded:
                BackColor = Color.FromArgb(75, 85, 99);    // gray 600
                _icon.Text = "↩️"; _icon.ForeColor = Color.White;
                _text.Text = "Refunded."; _text.ForeColor = Color.White;
                break;
        }
    }
}
