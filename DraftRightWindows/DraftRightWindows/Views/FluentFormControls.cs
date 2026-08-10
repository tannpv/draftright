using System;
using System.Windows;
using System.Windows.Controls;
using Wpf.Ui.Controls;
using WpfButton = Wpf.Ui.Controls.Button;
using WpfTextBox = Wpf.Ui.Controls.TextBox;
using WpfTextBlock = System.Windows.Controls.TextBlock;
using WpfPasswordBox = Wpf.Ui.Controls.PasswordBox;

namespace DraftRightWindows.Views;

/// <summary>
/// WPF-UI form-control factories shared by the settings tabs, the subscription
/// panel, and future migrated dialogs. Built once here so every surface spaces
/// and styles fields the same way — the WinForms version repeated Make* helpers
/// and hand-placed Point(x,y) coordinates per tab, which is exactly how the
/// invisible-capture-button bug happened. RULE #1: one source for the form
/// vocabulary; controls self-theme via the dictionaries FluentWindowBase merges.
/// </summary>
internal static class FluentFormControls
{
    // Layout spacing — one source each.
    public const double ContentPad = 24;    // window/tab content padding
    public const double SectionGap = 18;     // between sections
    public const double FieldGap = 12;       // between fields
    public const double LabelGap = 4;        // caption/label to its control
    public const double ButtonGap = 8;       // between adjacent buttons

    public static WpfTextBlock SectionHeader(string text) => new()
    {
        Text = text,
        FontSize = 16,
        FontWeight = FontWeights.SemiBold,
        Foreground = Theme.WpfBrush(Theme.TextPrimary),
        Margin = new Thickness(0, 0, 0, LabelGap),
    };

    public static WpfTextBlock FieldLabel(string text) => new()
    {
        Text = text,
        FontSize = 12,
        Foreground = Theme.WpfBrush(Theme.TextMuted),
        Margin = new Thickness(0, 0, 0, LabelGap),
    };

    public static WpfTextBlock Body(string text) => new()
    {
        Text = text,
        Foreground = Theme.WpfBrush(Theme.TextMuted),
        TextWrapping = TextWrapping.Wrap,
    };

    public static WpfTextBox TextField(string value = "") => new()
    {
        Text = value,
        Margin = new Thickness(0, 0, 0, FieldGap),
    };

    public static WpfPasswordBox PasswordField() => new()
    {
        Margin = new Thickness(0, 0, 0, FieldGap),
    };

    public static ComboBox Dropdown(bool editable = false) => new()
    {
        IsEditable = editable,
        Margin = new Thickness(0, 0, 0, FieldGap),
    };

    public static CheckBox Check(string text, bool isChecked) => new()
    {
        Content = text,
        IsChecked = isChecked,
        Margin = new Thickness(0, 0, 0, FieldGap),
    };

    public static WpfButton PrimaryButton(string text, Action onClick) =>
        Button(text, ControlAppearance.Primary, onClick);

    public static WpfButton SecondaryButton(string text, Action onClick) =>
        Button(text, ControlAppearance.Secondary, onClick);

    // MinWidth + padding so labels never clip (the WPF bug-dialog buttons did
    // before this shape existed).
    public static WpfButton Button(string text, ControlAppearance appearance, Action onClick)
    {
        var b = new WpfButton
        {
            Content = text,
            Appearance = appearance,
            MinWidth = 96,
            Padding = new Thickness(14, 6, 14, 6),
        };

        // Primary buttons rely on WPF-UI's accent brush, which is unset here:
        // there is no WPF Application (the app is a WinUI host running each
        // window on its own thread), so nothing populates the accent resources
        // and Primary renders black. Give it our brand blue explicitly — an
        // inline Background beats the appearance style, so the button is brand-
        // coloured everywhere (#reported: black Submit button).
        if (appearance == ControlAppearance.Primary)
        {
            b.Background = Theme.WpfBrush(Theme.BrandBlue);
            b.Foreground = System.Windows.Media.Brushes.White;
        }

        b.Click += (_, _) => onClick();
        return b;
    }
}
