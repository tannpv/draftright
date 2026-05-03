using System;
using System.Collections.Generic;
using System.Linq;
using DraftRightWindows.Models;
using Microsoft.UI;
using Microsoft.UI.Text;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Documents;
using Microsoft.UI.Xaml.Media;

namespace DraftRightWindows.Views;

/// <summary>
/// Structured Grammar Check result view, ported from the macOS GrammarCheckView.
/// Shows a score badge, issue counter, "Fix All" button, the original text with
/// per-issue highlights, and one issue card per finding with its own Fix button.
///
/// Important: applying a fix uses content-anchored resolution
/// (search for Original near Offset) instead of trusting the LLM offset.
/// LLMs are reliably wrong about character positions on multi-line input.
/// </summary>
public sealed class GrammarCheckView : UserControl
{
    // Brand palette — matches RewritePanel.cs.
    private static readonly Windows.UI.Color CardBgColor       = Windows.UI.Color.FromArgb(255, 30, 41, 59);
    private static readonly Windows.UI.Color BorderColor       = Windows.UI.Color.FromArgb(255, 51, 65, 85);
    private static readonly Windows.UI.Color TextPrimaryColor  = Windows.UI.Color.FromArgb(255, 226, 232, 240);
    private static readonly Windows.UI.Color TextMutedColor    = Windows.UI.Color.FromArgb(255, 148, 163, 184);
    private static readonly Windows.UI.Color SectionBgColor    = Windows.UI.Color.FromArgb(255, 23, 31, 50);
    private static readonly Windows.UI.Color SuccessGreenColor = Windows.UI.Color.FromArgb(255, 16, 185, 129);
    private static readonly Windows.UI.Color WarnOrangeColor   = Windows.UI.Color.FromArgb(255, 245, 158, 11);
    private static readonly Windows.UI.Color BadRedColor       = Windows.UI.Color.FromArgb(255, 239, 68, 68);
    private static readonly Windows.UI.Color StyleBlueColor    = Windows.UI.Color.FromArgb(255, 93, 135, 255);

    private readonly string _originalText;
    private readonly Action<string>? _onReplace;
    private readonly Action<string>? _onCopy;

    private string _currentText;
    private List<GrammarIssue> _remainingIssues;

    private TextBlock _scoreText = null!;
    private TextBlock _issueCountText = null!;
    private Button _fixAllButton = null!;
    private RichTextBlock _highlightedText = null!;
    private StackPanel _issueCardsPanel = null!;
    private StackPanel _allClearPanel = null!;
    private ScrollViewer _scroll = null!;

    public GrammarCheckView(
        string originalText,
        GrammarResult result,
        Action<string>? onReplace = null,
        Action<string>? onCopy = null)
    {
        _originalText = originalText;
        _currentText = originalText;
        _remainingIssues = new List<GrammarIssue>(result.Issues);
        _onReplace = onReplace;
        _onCopy = onCopy;

        Content = BuildUI(result.Score);
        Render();
    }

    /// <summary>Current resolved text (after any applied fixes). Hand to clipboard / replace caller.</summary>
    public string CurrentText => _currentText;

    private FrameworkElement BuildUI(int score)
    {
        var root = new Grid
        {
            Background = new SolidColorBrush(CardBgColor),
            BorderBrush = new SolidColorBrush(BorderColor),
            BorderThickness = new Thickness(1),
            CornerRadius = new CornerRadius(8),
            Padding = new Thickness(0),
        };
        root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        root.RowDefinitions.Add(new RowDefinition { Height = new GridLength(1, GridUnitType.Star) });

        // Header bar: score | issue count | Fix All
        var header = new Grid
        {
            Padding = new Thickness(12, 8, 12, 8),
        };
        header.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        header.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        header.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        header.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });

        _scoreText = new TextBlock
        {
            FontSize = 13,
            FontWeight = FontWeights.SemiBold,
            VerticalAlignment = VerticalAlignment.Center,
            Foreground = new SolidColorBrush(ScoreColor(score)),
            Text = $"{score}/100",
        };
        Grid.SetColumn(_scoreText, 0);

        _issueCountText = new TextBlock
        {
            FontSize = 11,
            Foreground = new SolidColorBrush(TextMutedColor),
            VerticalAlignment = VerticalAlignment.Center,
            HorizontalAlignment = HorizontalAlignment.Right,
            Margin = new Thickness(0, 0, 8, 0),
        };
        Grid.SetColumn(_issueCountText, 2);

        _fixAllButton = new Button
        {
            Content = "Fix All",
            FontSize = 11,
            Padding = new Thickness(10, 4, 10, 4),
        };
        _fixAllButton.Click += (_, _) => FixAll();
        Grid.SetColumn(_fixAllButton, 3);

        header.Children.Add(_scoreText);
        header.Children.Add(_issueCountText);
        header.Children.Add(_fixAllButton);

        // Divider
        var divider = new Border
        {
            Background = new SolidColorBrush(BorderColor),
            Height = 1,
            Margin = new Thickness(12, 0, 12, 0),
        };

        // Body — scrollable. Two children swap based on whether any issues remain.
        _scroll = new ScrollViewer
        {
            HorizontalScrollMode = ScrollMode.Disabled,
            VerticalScrollMode = ScrollMode.Auto,
            Padding = new Thickness(12),
        };

        var body = new StackPanel { Spacing = 12 };

        // Highlighted text section — visible while issues remain
        var highlightedSection = new StackPanel { Spacing = 4 };
        highlightedSection.Children.Add(new TextBlock
        {
            Text = "YOUR TEXT",
            FontSize = 10,
            FontWeight = FontWeights.SemiBold,
            Foreground = new SolidColorBrush(TextMutedColor),
        });

        _highlightedText = new RichTextBlock
        {
            FontSize = 13,
            LineHeight = 20,
            Foreground = new SolidColorBrush(TextPrimaryColor),
            IsTextSelectionEnabled = true,
        };
        var highlightedWrap = new Border
        {
            Background = new SolidColorBrush(SectionBgColor),
            CornerRadius = new CornerRadius(6),
            Padding = new Thickness(10),
            Child = _highlightedText,
        };
        highlightedSection.Children.Add(highlightedWrap);
        body.Children.Add(highlightedSection);

        // Issues found section
        var issuesHeader = new TextBlock
        {
            Text = "ISSUES FOUND",
            FontSize = 10,
            FontWeight = FontWeights.SemiBold,
            Foreground = new SolidColorBrush(TextMutedColor),
        };
        body.Children.Add(issuesHeader);

        _issueCardsPanel = new StackPanel { Spacing = 4 };
        body.Children.Add(_issueCardsPanel);

        // All-clear section — visible when remainingIssues is empty
        _allClearPanel = new StackPanel
        {
            Spacing = 8,
            Visibility = Visibility.Collapsed,
        };
        var checkRow = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 6 };
        checkRow.Children.Add(new FontIcon
        {
            Glyph = "", // CheckMark
            FontSize = 14,
            Foreground = new SolidColorBrush(SuccessGreenColor),
        });
        var statusText = new TextBlock
        {
            Name = "AllClearStatus",
            FontSize = 12,
            FontWeight = FontWeights.SemiBold,
            Foreground = new SolidColorBrush(SuccessGreenColor),
        };
        checkRow.Children.Add(statusText);
        _allClearPanel.Children.Add(checkRow);

        var allClearTextBlock = new TextBlock
        {
            Name = "AllClearText",
            FontSize = 13,
            LineHeight = 20,
            Foreground = new SolidColorBrush(TextPrimaryColor),
            IsTextSelectionEnabled = true,
        };
        _allClearPanel.Children.Add(allClearTextBlock);
        body.Children.Add(_allClearPanel);

        _scroll.Content = body;

        Grid.SetRow(header, 0);
        Grid.SetRow(divider, 1);
        Grid.SetRow(_scroll, 2);
        root.Children.Add(header);
        root.Children.Add(divider);
        root.Children.Add(_scroll);

        return root;
    }

    private void Render()
    {
        // Score color updates live as fixes are applied (it doesn't recompute,
        // but we keep the badge consistent with the original score for now).

        // Issue counter
        var n = _remainingIssues.Count;
        _issueCountText.Text = n == 0
            ? "All clear"
            : $"{n} issue{(n == 1 ? "" : "s")}";

        _fixAllButton.Visibility = n == 0 ? Visibility.Collapsed : Visibility.Visible;

        if (n == 0)
        {
            // All-clear state
            _scroll.Content = BuildAllClearBody();
        }
        else
        {
            BuildHighlightedText();
            BuildIssueCards();
        }
    }

    private FrameworkElement BuildAllClearBody()
    {
        var stack = new StackPanel { Spacing = 8 };

        var row = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 6 };
        row.Children.Add(new FontIcon
        {
            Glyph = "",
            FontSize = 14,
            Foreground = new SolidColorBrush(SuccessGreenColor),
        });
        row.Children.Add(new TextBlock
        {
            Text = _currentText == _originalText
                ? "Your writing looks great!"
                : "All issues fixed!",
            FontSize = 12,
            FontWeight = FontWeights.SemiBold,
            Foreground = new SolidColorBrush(SuccessGreenColor),
        });
        stack.Children.Add(row);

        stack.Children.Add(new TextBlock
        {
            Text = _currentText,
            FontSize = 13,
            LineHeight = 20,
            Foreground = new SolidColorBrush(TextPrimaryColor),
            IsTextSelectionEnabled = true,
            TextWrapping = TextWrapping.Wrap,
        });

        return stack;
    }

    private void BuildHighlightedText()
    {
        // Build a single Paragraph with Run segments — one Run per text
        // chunk, with issue-typed Runs getting a colored background and
        // underline. Resolve issue ranges by content (don't trust offsets).
        var paragraph = new Paragraph();

        var ranges = ResolveIssueRanges();  // sorted, non-overlapping
        var cursor = 0;
        foreach (var (start, end, color) in ranges)
        {
            if (start > cursor)
            {
                paragraph.Inlines.Add(new Run
                {
                    Text = _currentText.Substring(cursor, start - cursor),
                });
            }
            var issueRun = new Run
            {
                Text = _currentText.Substring(start, end - start),
                Foreground = new SolidColorBrush(color),
                TextDecorations = Windows.UI.Text.TextDecorations.Underline,
                FontWeight = FontWeights.SemiBold,
            };
            paragraph.Inlines.Add(issueRun);
            cursor = end;
        }
        if (cursor < _currentText.Length)
        {
            paragraph.Inlines.Add(new Run { Text = _currentText.Substring(cursor) });
        }

        _highlightedText.Blocks.Clear();
        _highlightedText.Blocks.Add(paragraph);
    }

    /// <summary>
    /// Walk the remaining issues, find each Original in the current text near
    /// the claimed Offset, and return non-overlapping (start, end, color)
    /// ranges sorted by position.
    /// </summary>
    private List<(int Start, int End, Windows.UI.Color Color)> ResolveIssueRanges()
    {
        var ranges = new List<(int, int, Windows.UI.Color)>();
        foreach (var issue in _remainingIssues)
        {
            var range = ResolveIssueRange(issue);
            if (range.HasValue)
            {
                ranges.Add((range.Value.start, range.Value.end, ColorForType(issue.Type)));
            }
        }
        // Sort + drop overlaps (earlier wins).
        ranges.Sort((a, b) => a.Item1.CompareTo(b.Item1));
        var result = new List<(int, int, Windows.UI.Color)>();
        var lastEnd = 0;
        foreach (var r in ranges)
        {
            if (r.Item1 < lastEnd) continue;
            result.Add(r);
            lastEnd = r.Item2;
        }
        return result;
    }

    private (int start, int end)? ResolveIssueRange(GrammarIssue issue)
    {
        if (string.IsNullOrEmpty(issue.Original)) return null;

        // Strategy: find all occurrences of Original; pick the one closest to Offset.
        var candidates = new List<int>();
        var idx = 0;
        while (idx < _currentText.Length)
        {
            var found = _currentText.IndexOf(issue.Original, idx, StringComparison.Ordinal);
            if (found < 0) break;
            candidates.Add(found);
            idx = found + 1;
        }
        if (candidates.Count == 0) return null;

        var best = candidates.OrderBy(c => Math.Abs(c - issue.Offset)).First();
        return (best, best + issue.Original.Length);
    }

    private void BuildIssueCards()
    {
        _issueCardsPanel.Children.Clear();
        foreach (var issue in _remainingIssues)
        {
            _issueCardsPanel.Children.Add(BuildIssueCard(issue));
        }
    }

    private FrameworkElement BuildIssueCard(GrammarIssue issue)
    {
        var color = ColorForType(issue.Type);

        var card = new Border
        {
            Background = new SolidColorBrush(SectionBgColor),
            CornerRadius = new CornerRadius(6),
            Padding = new Thickness(10, 8, 10, 8),
        };

        var grid = new Grid();
        grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        grid.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });

        var content = new StackPanel { Spacing = 4 };

        // Type label + reason
        var topRow = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 4 };
        topRow.Children.Add(new TextBlock
        {
            Text = LabelForType(issue.Type),
            FontSize = 10,
            FontWeight = FontWeights.SemiBold,
            Foreground = new SolidColorBrush(color),
        });
        if (!string.IsNullOrEmpty(issue.Reason))
        {
            topRow.Children.Add(new TextBlock
            {
                Text = "·",
                FontSize = 10,
                Foreground = new SolidColorBrush(TextMutedColor),
            });
            topRow.Children.Add(new TextBlock
            {
                Text = issue.Reason,
                FontSize = 10,
                Foreground = new SolidColorBrush(TextMutedColor),
                TextTrimming = TextTrimming.CharacterEllipsis,
                MaxLines = 1,
            });
        }
        content.Children.Add(topRow);

        // Original → Suggestion
        var fixRow = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 6 };
        fixRow.Children.Add(MakeChip(issue.Original, color, struckThrough: true, muted: true));
        fixRow.Children.Add(new FontIcon
        {
            Glyph = "", // Forward arrow
            FontSize = 9,
            Foreground = new SolidColorBrush(TextMutedColor),
            VerticalAlignment = VerticalAlignment.Center,
        });
        fixRow.Children.Add(MakeChip(issue.Suggestion, color, struckThrough: false, muted: false));
        content.Children.Add(fixRow);

        Grid.SetColumn(content, 0);
        grid.Children.Add(content);

        var fixButton = new Button
        {
            Content = "Fix",
            FontSize = 11,
            FontWeight = FontWeights.SemiBold,
            Padding = new Thickness(10, 2, 10, 2),
            VerticalAlignment = VerticalAlignment.Top,
        };
        fixButton.Click += (_, _) => ApplyFix(issue);
        Grid.SetColumn(fixButton, 1);
        grid.Children.Add(fixButton);

        card.Child = grid;
        return card;
    }

    private FrameworkElement MakeChip(string text, Windows.UI.Color color, bool struckThrough, bool muted)
    {
        var fillBg = Windows.UI.Color.FromArgb(20, color.R, color.G, color.B);
        var run = new Run
        {
            Text = text,
            Foreground = new SolidColorBrush(muted ? TextMutedColor : color),
            FontSize = 12,
            FontWeight = muted ? FontWeights.Normal : FontWeights.SemiBold,
        };
        if (struckThrough)
        {
            run.TextDecorations = Windows.UI.Text.TextDecorations.Strikethrough;
        }
        var rtb = new RichTextBlock();
        var p = new Paragraph();
        p.Inlines.Add(run);
        rtb.Blocks.Add(p);

        return new Border
        {
            Background = new SolidColorBrush(fillBg),
            CornerRadius = new CornerRadius(3),
            Padding = new Thickness(4, 2, 4, 2),
            Child = rtb,
        };
    }

    // ── Fix actions ──

    private void ApplyFix(GrammarIssue issue)
    {
        var range = ResolveIssueRange(issue);
        if (!range.HasValue) return;
        var (start, end) = range.Value;
        _currentText = string.Concat(
            _currentText.AsSpan(0, start),
            issue.Suggestion,
            _currentText.AsSpan(end));
        _remainingIssues.Remove(issue);
        Render();
        _onReplace?.Invoke(_currentText);
    }

    private void FixAll()
    {
        // Apply right-to-left so earlier offsets stay valid as we shrink/grow text.
        var ordered = _remainingIssues
            .Select(issue => (issue, range: ResolveIssueRange(issue)))
            .Where(t => t.range.HasValue)
            .OrderByDescending(t => t.range!.Value.start)
            .ToList();

        foreach (var (issue, range) in ordered)
        {
            var (start, end) = range!.Value;
            _currentText = string.Concat(
                _currentText.AsSpan(0, start),
                issue.Suggestion,
                _currentText.AsSpan(end));
        }
        _remainingIssues.Clear();
        Render();
        _onReplace?.Invoke(_currentText);
    }

    // ── Color/label helpers ──

    private static Windows.UI.Color ScoreColor(int score) =>
        score >= 90 ? SuccessGreenColor :
        score >= 70 ? WarnOrangeColor :
                      BadRedColor;

    private static Windows.UI.Color ColorForType(string type) => type switch
    {
        "spelling" => BadRedColor,
        "grammar" => WarnOrangeColor,
        "style" => StyleBlueColor,
        _ => Windows.UI.Color.FromArgb(255, 156, 163, 175),
    };

    private static string LabelForType(string type) => type switch
    {
        "spelling" => "Spelling",
        "grammar" => "Grammar",
        "style" => "Style",
        _ => "Issue",
    };
}
