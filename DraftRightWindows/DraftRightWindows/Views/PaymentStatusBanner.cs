using System;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using System.Windows.Threading;
using DraftRightWindows.Models;
using WpfTextBlock = System.Windows.Controls.TextBlock;

namespace DraftRightWindows.Views;

/// <summary>
/// Compact live-status banner (WPF, #160) used inside <see cref="QrCheckoutDialog"/>
/// and <see cref="BankTransferDialog"/>. Subscribes to the payment-status stream
/// and renders pending → confirmed/failed/expired, auto-closing the parent via
/// <c>onConfirmed</c> after <see cref="AutoDismissDelay"/> on success.
/// </summary>
public sealed class PaymentStatusBanner : UserControl, IObserver<PaymentStatusUpdate>
{
    public TimeSpan AutoDismissDelay { get; set; } = TimeSpan.FromSeconds(2);

    private readonly WpfTextBlock _icon;
    private readonly WpfTextBlock _text;
    private readonly Border _bar;
    private readonly DispatcherTimer _autoDismiss;
    private readonly Action _onConfirmed;
    private IDisposable? _subscription;

    public PaymentStatusBanner(IObservable<PaymentStatusUpdate>? stream, Action onConfirmed)
    {
        _onConfirmed = onConfirmed;

        _icon = new WpfTextBlock { FontSize = 15, VerticalAlignment = VerticalAlignment.Center, Margin = new Thickness(0, 0, 8, 0) };
        _text = new WpfTextBlock { FontWeight = FontWeights.Bold, Foreground = Brushes.White, TextWrapping = TextWrapping.Wrap, VerticalAlignment = VerticalAlignment.Center };
        var row = new StackPanel { Orientation = System.Windows.Controls.Orientation.Horizontal };
        row.Children.Add(_icon);
        row.Children.Add(_text);
        _bar = new Border { Padding = new Thickness(10, 6, 10, 6), Child = row };
        Content = _bar;

        _autoDismiss = new DispatcherTimer { Interval = AutoDismissDelay };
        _autoDismiss.Tick += (_, _) => { _autoDismiss.Stop(); _onConfirmed(); };

        if (stream is null)
        {
            Visibility = Visibility.Collapsed;
        }
        else
        {
            ApplyStatus(PaymentStatus.Pending);
            _subscription = stream.Subscribe(this);
            Unloaded += (_, _) => _subscription?.Dispose();
        }
    }

    public void OnNext(PaymentStatusUpdate u)
    {
        if (!Dispatcher.CheckAccess()) { try { Dispatcher.BeginInvoke(new Action(() => OnNext(u))); } catch { } return; }
        ApplyStatus(u.Status);
        if (u.Status.IsSuccess())
        {
            _autoDismiss.Stop();
            _autoDismiss.Interval = AutoDismissDelay <= TimeSpan.Zero ? TimeSpan.FromMilliseconds(50) : AutoDismissDelay;
            _autoDismiss.Start();
        }
    }

    public void OnCompleted() { }
    public void OnError(Exception error) { }

    // Status → colour + icon + copy. One place, mirrors the WinForms banner.
    private void ApplyStatus(PaymentStatus status)
    {
        (Color bg, string icon, string text) = status switch
        {
            PaymentStatus.Completed => (Color.FromRgb(22, 101, 52), "✅", "Payment confirmed!"),
            PaymentStatus.Failed    => (Color.FromRgb(153, 27, 27), "❌", "Payment failed. Please try again."),
            PaymentStatus.Expired   => (Color.FromRgb(154, 52, 18), "⌛", "Took too long. If you already paid, check Subscription in a minute."),
            PaymentStatus.Refunded  => (Color.FromRgb(75, 85, 99), "↩", "Refunded."),
            _                       => (Color.FromRgb(30, 64, 175), "⏳", "Waiting for payment…"),
        };
        _bar.Background = new SolidColorBrush(bg);
        _icon.Text = icon;
        _text.Text = text;
    }
}

/// <summary>Shared read-only field grid rendered by the QR and bank-transfer
/// dialogs (WPF, #160). One implementation so the two never drift.</summary>
internal static class BankInfoTable
{
    public static void Render(Panel host, BankInfo info)
    {
        Row(host, "Bank", info.BankName, copyable: false);
        Row(host, "Account #", info.AccountNumber, copyable: true);
        Row(host, "Name", info.AccountName, copyable: false);
        Row(host, "Amount", $"{AmountString(info.Amount)} {info.Currency}", copyable: true);
        Row(host, "Reference", info.Reference, copyable: true,
            hint: "Must include this in the transfer description.");
    }

    private static void Row(Panel host, string label, string value, bool copyable, string? hint = null)
    {
        var grid = new Grid { Margin = new Thickness(0, 0, 0, 6) };
        grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(90) });
        grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        grid.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });

        var l = new WpfTextBlock { Text = label, Foreground = Theme.WpfBrush(Theme.TextMuted), VerticalAlignment = VerticalAlignment.Center };
        var box = new Wpf.Ui.Controls.TextBox
        {
            Text = value,
            IsReadOnly = true,
            FontFamily = new FontFamily("Cascadia Code, Consolas, monospace"),
            Margin = new Thickness(8, 0, 8, 0),
        };
        Grid.SetColumn(l, 0); Grid.SetColumn(box, 1);
        grid.Children.Add(l); grid.Children.Add(box);

        if (copyable)
        {
            var btn = FluentFormControls.SecondaryButton("Copy", () =>
            {
                try { System.Windows.Clipboard.SetText(value); } catch { /* clipboard busy */ }
            });
            btn.MinWidth = 64;
            Grid.SetColumn(btn, 2);
            grid.Children.Add(btn);
        }
        host.Children.Add(grid);

        if (!string.IsNullOrEmpty(hint))
        {
            host.Children.Add(new WpfTextBlock
            {
                Text = hint,
                FontStyle = FontStyles.Italic,
                FontSize = 12,
                Foreground = Theme.WpfBrush(Theme.TextMuted),
                Margin = new Thickness(98, 0, 0, 8),
                TextWrapping = TextWrapping.Wrap,
            });
        }
    }

    private static string AmountString(double v) =>
        v == Math.Floor(v) ? ((long)v).ToString() : v.ToString("F2");
}
