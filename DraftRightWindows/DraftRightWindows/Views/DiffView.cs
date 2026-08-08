using System.Collections.Generic;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Documents;
using DraftRightWindows.Diff;

namespace DraftRightWindows.Views;

/// <summary>
/// Side-by-side before/after diff in WPF — the migrated counterpart of
/// <see cref="DiffPanel"/> (#156). Original on the left with deletions
/// highlighted, rewritten on the right with insertions highlighted.
///
/// Presentation only. The diffing is <see cref="WordDiff"/>, already unit
/// tested and verified byte-identical to the macOS and Linux implementations,
/// so this class must not reimplement any of it.
/// </summary>
internal sealed class DiffView : Grid
{
    private readonly RichTextBox _left;
    private readonly RichTextBox _right;

    public DiffView()
    {
        ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });

        _left = BuildPane("Original", column: 0);
        _right = BuildPane("Rewritten", column: 1);
    }

    /// <summary>Renders the diff of <paramref name="original"/> → <paramref name="rewritten"/>.</summary>
    public void ShowDiff(string original, string rewritten)
    {
        var (oldTokens, newTokens) = WordDiff.Diff(original ?? string.Empty, rewritten ?? string.Empty);
        Render(_left, oldTokens, DiffKind.Deleted, Theme.DiffDeletedBg);
        Render(_right, newTokens, DiffKind.Inserted, Theme.DiffInsertedBg);
    }

    private RichTextBox BuildPane(string caption, int column)
    {
        var stack = new StackPanel { Margin = new Thickness(2) };
        stack.Children.Add(new TextBlock
        {
            Text = caption,
            FontSize = 11,
            FontWeight = FontWeights.SemiBold,
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            Margin = new Thickness(2, 0, 0, 4),
        });

        var box = new RichTextBox
        {
            IsReadOnly = true,
            Background = Theme.WpfBrush(Theme.CardBg),
            Foreground = Theme.WpfBrush(Theme.TextMuted),
            BorderThickness = new Thickness(1),
            VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
            // Word diffs wrap; a horizontal scrollbar would hide changes.
            HorizontalScrollBarVisibility = ScrollBarVisibility.Disabled,
            MinHeight = 140,
        };
        stack.Children.Add(box);

        SetColumn(stack, column);
        Children.Add(stack);
        return box;
    }

    /// <summary>
    /// Writes tokens into a pane, tinting only the runs of
    /// <paramref name="highlight"/>. Tokens of the opposite kind never reach
    /// their own side — <see cref="WordDiff"/> puts deletions only in the old
    /// list and insertions only in the new — so each side reads as continuous
    /// prose with the changes marked.
    /// </summary>
    private static void Render(RichTextBox box,
                               IReadOnlyList<DiffToken> tokens,
                               DiffKind highlight,
                               System.Drawing.Color highlightBg)
    {
        var paragraph = new Paragraph { Margin = new Thickness(0) };
        foreach (var token in tokens)
        {
            var run = new Run(token.Text);
            if (token.Kind == highlight)
            {
                run.Background = Theme.WpfBrush(highlightBg);
                run.Foreground = Theme.WpfBrush(Theme.TextPrimary);
            }
            paragraph.Inlines.Add(run);
        }
        box.Document = new FlowDocument(paragraph);
    }
}
