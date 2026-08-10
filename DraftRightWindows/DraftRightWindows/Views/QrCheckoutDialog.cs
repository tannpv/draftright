using System;
using System.IO;
using System.Net.Http;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media.Imaging;
using DraftRightWindows.Models;
using WpfTextBlock = System.Windows.Controls.TextBlock;
using WpfImage = System.Windows.Controls.Image;
using F = DraftRightWindows.Views.FluentFormControls;

namespace DraftRightWindows.Views;

/// <summary>
/// VietQR checkout dialog (WPF, #160). Renders the QR image and, when included,
/// the manual-transfer fallback fields via <see cref="BankInfoTable"/>. A live
/// <see cref="PaymentStatusBanner"/> auto-dismisses on confirmation.
/// </summary>
public sealed class QrCheckoutDialog : FluentWindowBase
{
    public QrCheckoutDialog(QrCheckout checkout, IObservable<PaymentStatusUpdate>? statusStream)
    {
        Title = "Scan to pay";
        Width = 460;
        Height = 640;
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
            Text = "Scan to pay",
            FontSize = 15,
            FontWeight = FontWeights.Bold,
            Foreground = Theme.WpfBrush(Theme.TextPrimary),
            Margin = new Thickness(0, 0, 0, F.LabelGap),
        });
        panel.Children.Add(new WpfTextBlock
        {
            Text = "Open your banking app and scan this QR code.\nYour plan activates automatically after payment.",
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            TextWrapping = TextWrapping.Wrap,
            Margin = new Thickness(0, 0, 0, F.FieldGap),
        });

        var qr = new WpfImage { Width = 260, Height = 260, HorizontalAlignment = HorizontalAlignment.Center, Stretch = System.Windows.Media.Stretch.Uniform };
        var qrHost = new Border
        {
            Background = Theme.WpfBrush(Theme.CardBg),
            Width = 260,
            Height = 260,
            HorizontalAlignment = HorizontalAlignment.Center,
            Margin = new Thickness(0, 0, 0, F.SectionGap),
            Child = qr,
        };
        panel.Children.Add(qrHost);
        _ = LoadQrAsync(qr, qrHost, checkout.ImageUrl);

        if (checkout.BankInfo is { } bank)
        {
            panel.Children.Add(new WpfTextBlock
            {
                Text = "Or transfer manually",
                FontWeight = FontWeights.Bold,
                Foreground = Theme.WpfBrush(Theme.TextMuted),
                Margin = new Thickness(0, 0, 0, F.FieldGap),
            });
            BankInfoTable.Render(panel, bank);
        }

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

    private static async Task LoadQrAsync(WpfImage image, Border host, string url)
    {
        try
        {
            using var http = new HttpClient();
            var bytes = await http.GetByteArrayAsync(url);
            var bmp = new BitmapImage();
            using (var ms = new MemoryStream(bytes))
            {
                bmp.BeginInit();
                bmp.CacheOption = BitmapCacheOption.OnLoad;
                bmp.StreamSource = ms;
                bmp.EndInit();
            }
            bmp.Freeze();
            image.Source = bmp;
        }
        catch
        {
            host.Child = new WpfTextBlock
            {
                Text = "Could not load QR.\nUse manual transfer below.",
                Foreground = Theme.WpfBrush(Theme.TextPrimary),
                TextAlignment = TextAlignment.Center,
                TextWrapping = TextWrapping.Wrap,
                HorizontalAlignment = HorizontalAlignment.Center,
                VerticalAlignment = VerticalAlignment.Center,
            };
        }
    }
}
