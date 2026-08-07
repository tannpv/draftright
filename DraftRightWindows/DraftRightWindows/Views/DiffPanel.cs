using System.Drawing;
using DraftRightWindows.Diff;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows.Views;

/// <summary>
/// Side-by-side before/after view, mirroring macOS <c>DiffView.swift</c>
/// (issue #107): the original on the left with deletions highlighted, the
/// rewritten text on the right with insertions highlighted.
///
/// Only the diffing is interesting, and it lives in
/// <see cref="WordDiff"/> — already covered by DraftRightWindows.PureTests and
/// verified byte-identical to the macOS and Linux implementations. This class
/// is presentation only.
///
/// WinForms rather than WinUI: the rewrite panel is WinForms because WinUI
/// crashes on unpackaged builds when XAML theme resources cannot resolve
/// without a generated PRI. See the header of RewritePanelForm.
/// </summary>
internal sealed class DiffPanel : WinForms.Panel
{
    private readonly WinForms.RichTextBox _left;
    private readonly WinForms.RichTextBox _right;

    public DiffPanel()
    {
        Dock = WinForms.DockStyle.Fill;
        BackColor = Theme.BgDark;

        var grid = new WinForms.TableLayoutPanel
        {
            Dock = WinForms.DockStyle.Fill,
            ColumnCount = 2,
            RowCount = 2,
            BackColor = Theme.BgDark,
        };
        grid.ColumnStyles.Add(new WinForms.ColumnStyle(WinForms.SizeType.Percent, 50f));
        grid.ColumnStyles.Add(new WinForms.ColumnStyle(WinForms.SizeType.Percent, 50f));
        grid.RowStyles.Add(new WinForms.RowStyle(WinForms.SizeType.AutoSize));
        grid.RowStyles.Add(new WinForms.RowStyle(WinForms.SizeType.Percent, 100f));

        grid.Controls.Add(Caption("Original"), 0, 0);
        grid.Controls.Add(Caption("Rewritten"), 1, 0);

        _left = Pane();
        _right = Pane();
        grid.Controls.Add(_left, 0, 1);
        grid.Controls.Add(_right, 1, 1);

        Controls.Add(grid);
    }

    /// <summary>Renders the diff of <paramref name="original"/> → <paramref name="rewritten"/>.</summary>
    public void Show(string original, string rewritten)
    {
        var (oldTokens, newTokens) = WordDiff.Diff(original ?? string.Empty, rewritten ?? string.Empty);
        Render(_left, oldTokens, DiffKind.Deleted, Theme.DiffDeletedBg);
        Render(_right, newTokens, DiffKind.Inserted, Theme.DiffInsertedBg);
    }

    /// <summary>
    /// Writes tokens into a pane, tinting only the runs of
    /// <paramref name="highlight"/>. Tokens of the opposite kind never reach
    /// their own side — <see cref="WordDiff"/> puts deletions only in the old
    /// list and insertions only in the new one — so each side reads as
    /// continuous prose with the changes marked, matching macOS.
    /// </summary>
    private static void Render(WinForms.RichTextBox box,
                               System.Collections.Generic.IReadOnlyList<DiffToken> tokens,
                               DiffKind highlight,
                               Color highlightBg)
    {
        box.Clear();
        foreach (var token in tokens)
        {
            box.SelectionStart = box.TextLength;
            box.SelectionLength = 0;
            var isChange = token.Kind == highlight;
            box.SelectionBackColor = isChange ? highlightBg : Theme.CardBg;
            box.SelectionColor = isChange ? Theme.TextPrimary : Theme.TextMuted;
            box.AppendText(token.Text);
        }
        // Leave the caret at the top; AppendText scrolls to the end otherwise
        // and the user would open the panel looking at the tail of the text.
        box.SelectionStart = 0;
        box.ScrollToCaret();
    }

    private static WinForms.Label Caption(string text) => new()
    {
        Text = text,
        ForeColor = Theme.TextMuted,
        Font = new Font("Segoe UI", 8.5f, FontStyle.Bold),
        AutoSize = true,
        Margin = new WinForms.Padding(4, 2, 0, 2),
    };

    private static WinForms.RichTextBox Pane() => new()
    {
        ReadOnly = true,
        Multiline = true,
        BackColor = Theme.CardBg,
        ForeColor = Theme.TextMuted,
        BorderStyle = WinForms.BorderStyle.FixedSingle,
        Font = new Font("Segoe UI", 10f),
        Dock = WinForms.DockStyle.Fill,
        Margin = new WinForms.Padding(2),
        // Word-level diffs wrap; a horizontal scrollbar would hide changes.
        WordWrap = true,
        ScrollBars = WinForms.RichTextBoxScrollBars.Vertical,
        DetectUrls = false,
    };
}
