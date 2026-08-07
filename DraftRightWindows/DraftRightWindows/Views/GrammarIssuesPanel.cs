using System;
using System.Collections.Generic;
using System.Drawing;
using DraftRightWindows.Models;
using DraftRightWindows.Services;
using WinForms = System.Windows.Forms;

namespace DraftRightWindows.Views;

/// <summary>
/// Grammar-check results: a score, then one card per issue with its own Fix
/// button, plus Fix All. Mirrors macOS <c>GrammarCheckView.swift</c> (#107).
///
/// Every range comes from <see cref="GrammarFixer"/>, which resolves by the
/// issue's <c>Original</c> CONTENT rather than the LLM's offset — trusting
/// offsets spliced suggestions into the middle of words (BR#49). That logic is
/// unit-tested in DraftRightWindows.PureTests; this class is presentation and
/// wiring only.
///
/// After each fix the remaining issues are re-resolved against the new text, so
/// stale ones (whose anchor an overlapping fix removed) disappear on their own
/// and no offset bookkeeping is needed.
/// </summary>
internal sealed class GrammarIssuesPanel : WinForms.Panel
{
    /// <summary>Raised whenever the text changes, so the host can update its
    /// output box and enable Replace/Copy against the corrected text.</summary>
    public event EventHandler<string>? TextChanged;

    private readonly WinForms.Label _header;
    private readonly WinForms.FlowLayoutPanel _list;
    private readonly WinForms.Button _fixAllBtn;

    private string _text = string.Empty;
    private List<GrammarIssue> _issues = new();

    public GrammarIssuesPanel()
    {
        Dock = WinForms.DockStyle.Fill;
        BackColor = Theme.BgDark;

        var root = new WinForms.TableLayoutPanel
        {
            Dock = WinForms.DockStyle.Fill,
            ColumnCount = 1,
            RowCount = 3,
            BackColor = Theme.BgDark,
        };
        root.RowStyles.Add(new WinForms.RowStyle(WinForms.SizeType.AutoSize));
        root.RowStyles.Add(new WinForms.RowStyle(WinForms.SizeType.Percent, 100f));
        root.RowStyles.Add(new WinForms.RowStyle(WinForms.SizeType.AutoSize));

        _header = new WinForms.Label
        {
            ForeColor = Theme.TextPrimary,
            Font = new Font("Segoe UI", 10f, FontStyle.Bold),
            AutoSize = true,
            Margin = new WinForms.Padding(4, 4, 0, 6),
        };
        root.Controls.Add(_header, 0, 0);

        _list = new WinForms.FlowLayoutPanel
        {
            Dock = WinForms.DockStyle.Fill,
            FlowDirection = WinForms.FlowDirection.TopDown,
            WrapContents = false,
            AutoScroll = true,
            BackColor = Theme.BgDark,
        };
        root.Controls.Add(_list, 0, 1);

        _fixAllBtn = new WinForms.Button
        {
            Text = "Fix All",
            BackColor = Theme.BrandBlue,
            ForeColor = Color.White,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 9.5f, FontStyle.Bold),
            AutoSize = true,
            Margin = new WinForms.Padding(4, 6, 0, 2),
        };
        _fixAllBtn.FlatAppearance.BorderSize = 0;
        _fixAllBtn.Click += (_, _) => ApplyAll();
        root.Controls.Add(_fixAllBtn, 0, 2);

        Controls.Add(root);
    }

    /// <summary>Loads a result. <paramref name="text"/> is the text the issues
    /// were found in, and the base every fix is applied to.</summary>
    public void Load(string text, GrammarResult result)
    {
        _text = text ?? string.Empty;
        // Drop anything that no longer resolves before rendering, so the list
        // never offers a fix that would silently do nothing.
        _issues = GrammarFixer.RemainingIssues(_text, result?.Issues ?? new List<GrammarIssue>());
        Rebuild(result?.Score ?? 0);
    }

    private void Rebuild(int score)
    {
        _header.Text = _issues.Count == 0
            ? $"Score {score}/100 — no issues found"
            : $"Score {score}/100 — {_issues.Count} issue{(_issues.Count == 1 ? "" : "s")}";

        _list.SuspendLayout();
        foreach (WinForms.Control c in _list.Controls) c.Dispose();
        _list.Controls.Clear();
        foreach (var issue in _issues) _list.Controls.Add(BuildCard(issue, score));
        _list.ResumeLayout();

        _fixAllBtn.Visible = _issues.Count > 0;
    }

    private WinForms.Control BuildCard(GrammarIssue issue, int score)
    {
        var type = GrammarIssueTypeExtensions.FromWire(issue.Type);

        var card = new WinForms.Panel
        {
            BackColor = Theme.CardBg,
            Width = Math.Max(320, _list.ClientSize.Width - 28),
            Height = 86,
            Margin = new WinForms.Padding(4, 0, 4, 6),
        };

        // Colour bar keys the card to its category — spelling red, grammar
        // orange, style blue, matching macOS and Linux. Parsed from the shared
        // hex so the three clients cannot drift.
        card.Controls.Add(new WinForms.Panel
        {
            BackColor = ColorTranslator.FromHtml(type.HexColor()),
            Dock = WinForms.DockStyle.Left,
            Width = 4,
        });

        card.Controls.Add(new WinForms.Label
        {
            Text = type.DisplayName().ToUpperInvariant(),
            ForeColor = ColorTranslator.FromHtml(type.HexColor()),
            Font = new Font("Segoe UI", 8f, FontStyle.Bold),
            AutoSize = true,
            Location = new Point(14, 8),
        });

        card.Controls.Add(new WinForms.Label
        {
            Text = $"{issue.Original}  →  {issue.Suggestion}",
            ForeColor = Theme.TextPrimary,
            Font = new Font("Segoe UI", 10f),
            AutoSize = false,
            Size = new Size(card.Width - 100, 22),
            Location = new Point(14, 26),
            AutoEllipsis = true,
        });

        card.Controls.Add(new WinForms.Label
        {
            Text = issue.Reason,
            ForeColor = Theme.TextMuted,
            Font = new Font("Segoe UI", 8.5f),
            AutoSize = false,
            Size = new Size(card.Width - 100, 30),
            Location = new Point(14, 50),
            AutoEllipsis = true,
        });

        var fix = new WinForms.Button
        {
            Text = "Fix",
            BackColor = Theme.BorderColor,
            ForeColor = Theme.TextPrimary,
            FlatStyle = WinForms.FlatStyle.Flat,
            Font = new Font("Segoe UI", 9f),
            Size = new Size(64, 26),
            Location = new Point(card.Width - 78, 30),
            Anchor = WinForms.AnchorStyles.Top | WinForms.AnchorStyles.Right,
        };
        fix.FlatAppearance.BorderSize = 0;
        fix.Click += (_, _) => ApplyOne(issue, score);
        card.Controls.Add(fix);

        return card;
    }

    private void ApplyOne(GrammarIssue issue, int score)
    {
        _text = GrammarFixer.ApplyFix(_text, issue);
        // Re-resolve the survivors against the new text: applying one fix can
        // invalidate another whose anchor it overlapped.
        _issues = GrammarFixer.RemainingIssues(_text, Without(_issues, issue));
        Rebuild(score);
        TextChanged?.Invoke(this, _text);
    }

    private void ApplyAll()
    {
        _text = GrammarFixer.FixAll(_text, _issues);
        _issues = new List<GrammarIssue>();
        Rebuild(0);
        _header.Text = "All issues fixed";
        TextChanged?.Invoke(this, _text);
    }

    private static List<GrammarIssue> Without(List<GrammarIssue> issues, GrammarIssue drop)
    {
        var kept = new List<GrammarIssue>(issues.Count);
        foreach (var i in issues)
        {
            if (!ReferenceEquals(i, drop)) kept.Add(i);
        }
        return kept;
    }
}
