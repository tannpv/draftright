using System;
using System.Threading;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Threading;
using Wpf.Ui.Controls;
using WpfTextBlock = System.Windows.Controls.TextBlock;
using F = DraftRightWindows.Views.FluentFormControls;

namespace DraftRightWindows.Views;

/// <summary>
/// One-time "What's New in DraftRight vX" notice shown on the first launch after
/// an update applies (#160, WPF). Runs on its own STA thread with a WPF
/// dispatcher — same approach as the other detached windows. On the update path,
/// so it must stay reliable.
/// </summary>
public static class WhatsNewWindow
{
    /// <summary>Shows the notice on a dedicated STA thread and returns
    /// immediately. <paramref name="notes"/> is the release-note text from the
    /// backend; displayed verbatim.</summary>
    public static void Show(string version, string notes)
    {
        Services.StaWindowHost.Run("whats-new", () => new WhatsNewNotice(version, notes));
    }
}

internal sealed class WhatsNewNotice : FluentWindowBase
{
    public WhatsNewNotice(string version, string notes)
    {
        Title = "What's New";
        Width = 480;
        Height = 380;
        ResizeMode = ResizeMode.NoResize;
        Topmost = true;
        WindowStartupLocation = WindowStartupLocation.CenterScreen;

        var icon = Helpers.AppIcon.LoadImageSource();
        if (icon != null) Icon = icon;

        var panel = new StackPanel { Margin = new Thickness(F.ContentPad) };
        panel.Children.Add(new WpfTextBlock
        {
            Text = $"What's new in DraftRight v{version}",
            FontSize = 15,
            FontWeight = FontWeights.Bold,
            Foreground = Theme.WpfBrush(Theme.TextPrimary),
            Margin = new Thickness(0, 0, 0, F.FieldGap),
        });

        panel.Children.Add(new Border
        {
            Background = Theme.WpfBrush(Theme.CardBg),
            BorderBrush = Theme.WpfBrush(Theme.BorderColor),
            BorderThickness = new Thickness(1),
            CornerRadius = new CornerRadius(4),
            Height = 210,
            Margin = new Thickness(0, 0, 0, F.FieldGap),
            Child = new ScrollViewer
            {
                VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
                Padding = new Thickness(10),
                Content = new WpfTextBlock
                {
                    Text = string.IsNullOrWhiteSpace(notes) ? "Various improvements and fixes." : notes,
                    Foreground = Theme.WpfBrush(Theme.TextPrimary),
                    TextWrapping = TextWrapping.Wrap,
                },
            },
        });

        var gotIt = F.PrimaryButton("Got it", Close);
        gotIt.HorizontalAlignment = HorizontalAlignment.Right;
        panel.Children.Add(gotIt);

        Content = panel;
    }
}
