using System;
using System.Windows;
using System.Windows.Controls;
using DraftRightWindows.Services;
using Wpf.Ui.Controls;
using WpfButton = Wpf.Ui.Controls.Button;
using WpfTextBox = Wpf.Ui.Controls.TextBox;
using WpfTextBlock = System.Windows.Controls.TextBlock;
using F = DraftRightWindows.Views.FluentFormControls;

namespace DraftRightWindows.Views;

/// <summary>
/// "Suggest a feature" dialog (WPF, #160). Shares the shell — STA launcher,
/// identity block, status, submit flow — with <see cref="BugReportWindow"/> via
/// <see cref="FeedbackDialogBase"/>. Posts to <see cref="FeedbackService"/>.
/// </summary>
internal static class SuggestFeatureDialog
{
    public static void Show(IntPtr ownerHwnd = default) =>
        FeedbackDialogBase.RunOnStaThread(() => new SuggestFeatureWindow());
}

internal sealed class SuggestFeatureWindow : FeedbackDialogBase
{
    // Human label ↔ backend value for the platform dropdown. One seed list.
    private sealed record PlatformOpt(string Value, string Label)
    {
        public override string ToString() => Label;
    }

    private static readonly PlatformOpt[] Platforms =
    {
        new("playground", "Playground (web)"),
        new("mobile", "Mobile (iOS / Android)"),
        new("windows", "Windows"),
        new("mac", "macOS"),
        new("linux", "Linux"),
    };

    public SuggestFeatureWindow() : base("Suggest a feature")
    {
        var panel = new StackPanel { Margin = new Thickness(F.ContentPad) };

        panel.Children.Add(new WpfTextBlock
        {
            Text = "Got an idea? We'd love to hear it.",
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            TextWrapping = TextWrapping.Wrap,
            Margin = new Thickness(0, 0, 0, F.SectionGap),
        });

        panel.Children.Add(F.FieldLabel("Title"));
        var titleBox = new WpfTextBox { MaxLength = 80, Margin = new Thickness(0, 0, 0, F.FieldGap) };
        panel.Children.Add(titleBox);

        panel.Children.Add(F.FieldLabel("Target platform"));
        var platformCombo = F.Dropdown();
        foreach (var p in Platforms) platformCombo.Items.Add(p);
        platformCombo.SelectedIndex = 2; // Windows
        platformCombo.HorizontalAlignment = HorizontalAlignment.Left;
        platformCombo.MinWidth = 240;
        panel.Children.Add(platformCombo);

        panel.Children.Add(F.FieldLabel("Describe the feature"));
        var detailsBox = new WpfTextBox
        {
            MaxLength = 2000,
            AcceptsReturn = true,
            TextWrapping = TextWrapping.Wrap,
            VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
            Height = 120,
            Margin = new Thickness(0, 0, 0, F.FieldGap),
        };
        panel.Children.Add(detailsBox);

        var identity = BuildIdentityBlock("Submitting", out var readEmail);
        panel.Children.Add(identity);

        var seeAll = new HyperlinkButton
        {
            Content = "See all requests →",
            Margin = new Thickness(0, 0, 0, F.FieldGap),
        };
        seeAll.Click += (_, _) =>
        {
            try
            {
                System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo(
                    "https://draftright.info/feedback") { UseShellExecute = true });
            }
            catch { /* best-effort */ }
        };
        panel.Children.Add(seeAll);

        panel.Children.Add(StatusBar);

        var actions = BuildActions("Submit", out var submitBtn, out var cancelBtn);
        panel.Children.Add(actions);

        // Submit stays disabled until title + details are non-empty.
        void RefreshEnabled() =>
            submitBtn.IsEnabled = titleBox.Text.Trim().Length > 0 && detailsBox.Text.Trim().Length > 0;
        titleBox.TextChanged += (_, _) => RefreshEnabled();
        detailsBox.TextChanged += (_, _) => RefreshEnabled();
        RefreshEnabled();

        submitBtn.Click += async (_, _) =>
        {
            var titleText = titleBox.Text.Trim();
            var detailsText = detailsBox.Text.Trim();
            if (titleText.Length == 0) { SetStatus(InfoBarSeverity.Error, "Please enter a title."); titleBox.Focus(); return; }
            if (detailsText.Length == 0) { SetStatus(InfoBarSeverity.Error, "Please describe the feature."); detailsBox.Focus(); return; }

            var platform = ((PlatformOpt)platformCombo.SelectedItem!).Value;
            var email = readEmail();
            string? authToken = null;
            try { authToken = App.Auth?.AccessToken; } catch { authToken = null; }

            await RunSubmitAsync(submitBtn, cancelBtn,
                lockFields: () => { titleBox.IsReadOnly = detailsBox.IsReadOnly = true; platformCombo.IsEnabled = false; },
                unlockFields: () => { titleBox.IsReadOnly = detailsBox.IsReadOnly = false; platformCombo.IsEnabled = true; },
                successMessage: "Thanks! Your suggestion was submitted.",
                action: async () =>
                {
                    var result = await FeedbackService.SubmitAsync(
                        title: titleText,
                        targetPlatform: platform,
                        description: detailsText,
                        userEmail: !string.IsNullOrWhiteSpace(email) ? email : null,
                        authToken: authToken);
                    if (result.Success)
                        DRLogger.Log($"Feature request submitted (id={result.Id ?? "?"})", DRLogger.Category.APP);
                    return (result.Success, result.ErrorMessage);
                });
        };

        SetBody(new ScrollViewer
        {
            VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
            Content = panel,
        });
    }
}
