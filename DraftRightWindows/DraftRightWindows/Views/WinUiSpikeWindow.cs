using System;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using DraftRightWindows.Services;

namespace DraftRightWindows.Views;

/// <summary>
/// A throwaway WinUI 3 window that answers one question: does XAML content
/// render in an UNPACKAGED build, now that the installer regenerates
/// resources.pri?
///
/// Background. Every panel in this app is hand-drawn WinForms, not because
/// anyone preferred it but because a WinUI panel crashed on first render —
/// STATUS_STOWED_EXCEPTION 0xc000027b, "Cannot find a Resource …
/// TabViewScrollButtonBackground". Unpackaged builds had no resources.pri, so
/// the framework could not resolve themeresources.xaml (see App.cs).
///
/// Since then draftright.iss gained a post-install hook that copies
/// Microsoft.UI.Xaml.Controls.pri to resources.pri, and MSIX builds always had
/// a real one. If that is sufficient, the stated reason for WinForms is gone
/// and the UI can migrate to Fluent panel by panel.
///
/// Built in C# rather than a .xaml file on purpose: the project has no XAML
/// pages today, so adding one would drag in the XAML compiler and PRI page
/// generation and confuse the result. Control templates resolve theme
/// resources the same way either way, so this exercises the exact path that
/// crashed while changing nothing about the build.
///
/// DELETE THIS once the question is answered — either the migration starts, or
/// we settle on a different toolkit. It is not a feature.
/// </summary>
internal static class WinUiSpikeWindow
{
    /// <summary>
    /// Opens the spike. MUST be called on the WinUI dispatcher thread —
    /// constructing a Window elsewhere throws.
    /// </summary>
    public static void Show()
    {
        var window = new Window { Title = "WinUI 3 spike — is Fluent viable?" };

        var root = new StackPanel
        {
            Padding = new Thickness(24),
            Spacing = 16,
            // Deliberately a plain brush: Mica needs extra plumbing, and this
            // spike is about whether ANY XAML renders, not how pretty it is.
            Background = Theme.Brush(Theme.BgDark),
        };

        root.Children.Add(new TextBlock
        {
            Text = "WinUI 3 rendered",
            FontSize = 24,
            FontWeight = Microsoft.UI.Text.FontWeights.SemiBold,
            Foreground = Theme.Brush(Theme.TextPrimary),
        });

        root.Children.Add(new TextBlock
        {
            Text = "If you can read this, theme resources resolved and the "
                 + "WinForms fallback is no longer necessary.",
            TextWrapping = TextWrapping.Wrap,
            Foreground = Theme.Brush(Theme.TextMuted),
        });

        // Each control below pulls in a different slice of the theme
        // dictionaries. A partial failure — some render, one throws — is more
        // informative than a single control, so exercise a spread.
        root.Children.Add(new TextBox
        {
            PlaceholderText = "TextBox — templated input",
        });

        root.Children.Add(new ComboBox
        {
            PlaceholderText = "ComboBox — popup + flyout resources",
            Items = { "Polished", "Concise", "Technical" },
        });

        root.Children.Add(new ProgressRing
        {
            IsActive = true,
            Width = 32,
            Height = 32,
            HorizontalAlignment = HorizontalAlignment.Left,
        });

        var status = new TextBlock
        {
            Foreground = Theme.Brush(Theme.SuccessGreen),
            TextWrapping = TextWrapping.Wrap,
        };

        var button = new Button { Content = "Click me — proves input works too" };
        button.Click += (_, _) =>
            status.Text = $"Button clicked at {DateTime.Now:HH:mm:ss}. Interaction works.";
        root.Children.Add(button);
        root.Children.Add(status);

        // TabView is what the original crash named. Worth including precisely
        // because it is the control that failed — if everything else renders
        // and this one does not, that is the answer.
        var tabs = new TabView { Height = 120, IsAddTabButtonVisible = false };
        tabs.TabItems.Add(new TabViewItem
        {
            Header = "TabView",
            Content = new TextBlock
            {
                Text = "TabViewScrollButtonBackground was the missing resource.",
                Foreground = Theme.Brush(Theme.TextMuted),
                Margin = new Thickness(8),
            },
        });
        root.Children.Add(tabs);

        window.Content = root;
        window.Activate();
        DRLogger.Log("WinUI spike window activated — XAML content rendered without throwing",
                     DRLogger.Category.APP);
    }
}
