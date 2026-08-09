using System;
using System.Collections.Generic;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using DraftRightWindows.Models;
using DraftRightWindows.Services;
using WpfButton = Wpf.Ui.Controls.Button;

namespace DraftRightWindows.Views;

/// <summary>
/// Grammar-check results in WPF — the migrated counterpart of the old
/// WinForms <c>GrammarIssuesPanel</c> (removed in #157 once this replaced it).
/// Score, one card per issue colour-coded by type, each individually fixable,
/// plus Fix All.
///
/// Every range comes from <see cref="GrammarFixer"/>, which resolves by the
/// issue's <c>Original</c> CONTENT rather than the LLM's offset — trusting
/// offsets spliced suggestions into the middle of words (BR#49). That logic is
/// unit tested; this class is presentation and wiring only.
/// </summary>
internal sealed class GrammarIssuesView : Grid
{
    /// <summary>Raised with the corrected text after every fix, so the host can
    /// update its output and let Replace/Copy act on the correction.</summary>
    public event EventHandler<string>? TextChanged;

    private readonly TextBlock _header;
    private readonly StackPanel _list;
    private readonly WpfButton _fixAllBtn;

    private string _text = string.Empty;
    private List<GrammarIssue> _issues = new();
    private int _score;

    public GrammarIssuesView()
    {
        RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        RowDefinitions.Add(new RowDefinition { Height = new GridLength(1, GridUnitType.Star) });
        RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });

        _header = new TextBlock
        {
            FontSize = 14,
            FontWeight = FontWeights.SemiBold,
            Foreground = Theme.WpfBrush(Theme.TextPrimary),
            Margin = new Thickness(2, 0, 0, 8),
        };
        SetRow(_header, 0);
        Children.Add(_header);

        _list = new StackPanel();
        var scroll = new ScrollViewer
        {
            Content = _list,
            VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
        };
        SetRow(scroll, 1);
        Children.Add(scroll);

        _fixAllBtn = new WpfButton
        {
            Content = "Fix All",
            Appearance = Wpf.Ui.Controls.ControlAppearance.Primary,
            HorizontalAlignment = HorizontalAlignment.Right,
            Margin = new Thickness(0, 8, 0, 0),
        };
        _fixAllBtn.Click += (_, _) => ApplyAll();
        SetRow(_fixAllBtn, 2);
        Children.Add(_fixAllBtn);
    }

    /// <summary>Loads a result. <paramref name="text"/> is the text the issues
    /// were found in, and the base every fix applies to.</summary>
    public void Load(string text, GrammarResult result)
    {
        _text = text ?? string.Empty;
        _score = result?.Score ?? 0;
        // Drop anything that no longer resolves before rendering, so the list
        // never offers a fix that would silently do nothing.
        _issues = GrammarFixer.RemainingIssues(_text, result?.Issues ?? new List<GrammarIssue>());
        Rebuild();
    }

    private void Rebuild()
    {
        _header.Text = _issues.Count == 0
            ? $"Score {_score}/100 — no issues found"
            : $"Score {_score}/100 — {_issues.Count} issue{(_issues.Count == 1 ? "" : "s")}";

        _list.Children.Clear();
        foreach (var issue in _issues) _list.Children.Add(BuildCard(issue));
        _fixAllBtn.Visibility = _issues.Count > 0 ? Visibility.Visible : Visibility.Collapsed;
    }

    private UIElement BuildCard(GrammarIssue issue)
    {
        var type = GrammarIssueTypeExtensions.FromWire(issue.Type);
        var accent = (SolidColorBrush)new BrushConverter().ConvertFrom(type.HexColor())!;
        accent.Freeze();

        var card = new Border
        {
            Background = Theme.WpfBrush(Theme.CardBg),
            CornerRadius = new CornerRadius(6),
            Padding = new Thickness(10),
            Margin = new Thickness(0, 0, 0, 6),
            // Colour bar keys the card to its category — spelling red, grammar
            // orange, style blue, matching macOS and Linux via the shared enum.
            BorderBrush = accent,
            BorderThickness = new Thickness(4, 0, 0, 0),
        };

        var row = new Grid();
        row.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        row.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });

        var body = new StackPanel();
        body.Children.Add(new TextBlock
        {
            Text = type.DisplayName().ToUpperInvariant(),
            FontSize = 10,
            FontWeight = FontWeights.Bold,
            Foreground = accent,
        });
        body.Children.Add(new TextBlock
        {
            Text = $"{issue.Original}  →  {issue.Suggestion}",
            FontSize = 13,
            TextWrapping = TextWrapping.Wrap,
            Foreground = Theme.WpfBrush(Theme.TextPrimary),
            Margin = new Thickness(0, 2, 0, 0),
        });
        if (!string.IsNullOrWhiteSpace(issue.Reason))
        {
            body.Children.Add(new TextBlock
            {
                Text = issue.Reason,
                FontSize = 11,
                TextWrapping = TextWrapping.Wrap,
                Foreground = Theme.WpfBrush(Theme.TextMuted),
                Margin = new Thickness(0, 2, 0, 0),
            });
        }
        SetColumn(body, 0);
        row.Children.Add(body);

        var fix = new WpfButton
        {
            Content = "Fix",
            Appearance = Wpf.Ui.Controls.ControlAppearance.Secondary,
            VerticalAlignment = VerticalAlignment.Center,
            Margin = new Thickness(8, 0, 0, 0),
        };
        fix.Click += (_, _) => ApplyOne(issue);
        SetColumn(fix, 1);
        row.Children.Add(fix);

        card.Child = row;
        return card;
    }

    private void ApplyOne(GrammarIssue issue)
    {
        _text = GrammarFixer.ApplyFix(_text, issue);
        // Re-resolve the survivors against the new text: applying one fix can
        // invalidate another whose anchor it overlapped.
        var survivors = new List<GrammarIssue>();
        foreach (var i in _issues) if (!ReferenceEquals(i, issue)) survivors.Add(i);
        _issues = GrammarFixer.RemainingIssues(_text, survivors);
        Rebuild();
        TextChanged?.Invoke(this, _text);
    }

    private void ApplyAll()
    {
        _text = GrammarFixer.FixAll(_text, _issues);
        _issues = new List<GrammarIssue>();
        Rebuild();
        _header.Text = "All issues fixed";
        TextChanged?.Invoke(this, _text);
    }
}
