using System;
using System.Windows;
using System.Windows.Controls;
using DraftRightWindows.Models;
using WpfTextBlock = System.Windows.Controls.TextBlock;
using F = DraftRightWindows.Views.FluentFormControls;

namespace DraftRightWindows.Views;

/// <summary>
/// Bank-transfer checkout dialog (WPF, #160). Renders the account fields plus a
/// copyable reference code via <see cref="BankInfoTable"/>, and auto-dismisses
/// when the foreground poller reports success.
/// </summary>
public sealed class BankTransferDialog : FluentWindowBase
{
    public BankTransferDialog(BankTransferCheckout checkout, IObservable<PaymentStatusUpdate>? statusStream)
    {
        Title = "Bank transfer";
        Width = 460;
        Height = 400;
        ResizeMode = ResizeMode.NoResize;
        WindowStartupLocation = WindowStartupLocation.CenterOwner;

        var icon = Helpers.AppIcon.LoadImageSource();
        if (icon != null) Icon = icon;

        var outer = new DockPanel();

        var banner = new PaymentStatusBanner(statusStream, onConfirmed: Close);
        DockPanel.SetDock(banner, Dock.Top);
        outer.Children.Add(banner);

        var panel = new StackPanel { Margin = new Thickness(F.ContentPad) };
        panel.Children.Add(new WpfTextBlock
        {
            Text = "Bank transfer",
            FontSize = 15,
            FontWeight = FontWeights.Bold,
            Foreground = Theme.WpfBrush(Theme.TextPrimary),
            Margin = new Thickness(0, 0, 0, F.LabelGap),
        });
        panel.Children.Add(new WpfTextBlock
        {
            Text = "Transfer this exact amount from any Vietnamese bank. The reference code links the payment to your account; your plan activates automatically once received.",
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            TextWrapping = TextWrapping.Wrap,
            Margin = new Thickness(0, 0, 0, F.SectionGap),
        });

        BankInfoTable.Render(panel, checkout.Info);

        var close = F.SecondaryButton("Close", Close);
        close.HorizontalAlignment = HorizontalAlignment.Right;
        close.Margin = new Thickness(0, F.FieldGap, 0, 0);
        panel.Children.Add(close);

        outer.Children.Add(new ScrollViewer
        {
            VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
            Content = panel,
        });
        SetBody(outer);
    }
}
